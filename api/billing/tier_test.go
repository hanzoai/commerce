package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// tierBalanceResp mirrors the GetTier JSON `balance` block.
type tierBalanceResp struct {
	Balance struct {
		PrepaidAvailable   int64 `json:"prepaidAvailable"`
		DailyRemaining     int64 `json:"dailyRemaining"`
		EffectiveAvailable int64 `json:"effectiveAvailable"`
	} `json:"balance"`
}

// tierSeed scopes a request to org `ns` (org name == namespace), the exact
// plumbing the gateway/middleware injects, so GetTier resolves the ae SQLite
// datastore in that namespace.
func tierSeed(org *organization.Organization) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), org.Name))
	}
}

// callGetTier drives the GetTier handler and returns status + parsed balance.
func callGetTier(org *organization.Organization, user string) (int, tierBalanceResp) {
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/tier?user="+user, nil)
	w := driveSeeded(tierSeed(org), "/v1/billing/tier", req, GetTier)
	var resp tierBalanceResp
	_ = json.Unmarshal(func() []byte { b, _ := io.ReadAll(w.Body); return b }(), &resp)
	return w.StatusCode, resp
}

// seedTierDeposit writes a Deposit into org `ns`'s ledger, tagged `tag`, with
// the Test flag GetTier will filter on (org.TestMode()).
func seedTierDeposit(t *testing.T, ns string, testMode bool, user, tag string, cents int64) {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	tr := transaction.New(db)
	tr.Type = transaction.Deposit
	tr.DestinationId = user
	tr.DestinationKind = "iam-user"
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Tags = tag
	tr.Test = testMode
	tr.MustCreate()
}

// TestGetTier_FreeZeroBalance_Gated proves NO FREE TIER: a Free-tier user with
// zero transactions has effectiveAvailable == 0, so the metering client refuses
// (402). The old $1/day replenishing allowance no longer funds anyone.
func TestGetTier_FreeZeroBalance_Gated(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "tiergate-zero"

	code, resp := callGetTier(org, "tiergate-zero/alice")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if resp.Balance.DailyRemaining != 0 {
		t.Fatalf("dailyRemaining = %d, want 0 (no free daily allowance)", resp.Balance.DailyRemaining)
	}
	if resp.Balance.EffectiveAvailable != 0 {
		t.Fatalf("effectiveAvailable = %d, want 0 — a zero-balance account MUST be gated", resp.Balance.EffectiveAvailable)
	}
}

// TestGetTier_StarterCreditFunds proves the one-time starter grant is the ONLY
// onboarding funding: a lone $100 starter-credit Deposit yields
// effectiveAvailable == 10000 (the grant funds usage until spent). It shares the
// exact ledger the starter grant writes, so this is the real gate behavior.
func TestGetTier_StarterCreditFunds(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "tiergate-grant"
	const user = "tiergate-grant/bob"

	seedTierDeposit(t, org.Name, org.TestMode(), user, "starter-credit", 10000)

	code, resp := callGetTier(org, user)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if resp.Balance.EffectiveAvailable != 10000 {
		t.Fatalf("effectiveAvailable = %d, want 10000 — the one-time starter grant must fund usage", resp.Balance.EffectiveAvailable)
	}
	if resp.Balance.DailyRemaining != 0 {
		t.Fatalf("dailyRemaining = %d, want 0 (grant is prepaid balance, not a free daily allowance)", resp.Balance.DailyRemaining)
	}
}

// TestGetTier_StarterCreditSpent_Gates proves the gate closes once the one-time
// grant is spent: a $100 grant fully consumed by usage debits leaves
// effectiveAvailable == 0.
func TestGetTier_StarterCreditSpent_Gates(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{}
	org.Name = "tiergate-spent"
	const user = "tiergate-spent/carol"

	tm := org.TestMode()
	seedTierDeposit(t, org.Name, tm, user, "starter-credit", 10000)

	// Spend the entire grant via a usage withdrawal.
	db := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	w := transaction.New(db)
	w.Type = transaction.Withdraw
	w.SourceId = user
	w.SourceKind = "iam-user"
	w.Currency = currency.USD
	w.Amount = currency.Cents(10000)
	w.Tags = "api-usage"
	w.Test = tm
	w.MustCreate()

	code, resp := callGetTier(org, user)
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if resp.Balance.EffectiveAvailable != 0 {
		t.Fatalf("effectiveAvailable = %d, want 0 — a spent account MUST gate", resp.Balance.EffectiveAvailable)
	}
}

// A request that carries no organization must not panic.
//
// Measured live 2026-08-04: GET /v1/billing/tier answered 500 "internal server
// error" for every service-to-service caller, because GetTier opened with the
// one-value middleware.GetOrganization and the ai router's rate-limit lookup —
// this route's whole reason to exist — reaches it as S2S. IAMTokenRequired
// deliberately refuses to resolve an org from a bare X-Org-Id with no validated
// user identity and falls through, so Locals("organization") is nil there and
// the type assertion panicked into the recover handler.
//
// It must not answer Free either, which is why this asserts a 4xx and not a
// body: Free is what the router falls back to, and returning it from a request
// that could not read anything makes an unanswerable question look answered —
// every paying customer rate-limited to the most restrictive tier in the table,
// silently, which is the exact failure this route was added to fix.
func TestGetTier_NoOrganizationIsRefusedNotPanicked(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/tier?user=alice", nil)
	w := driveSeeded(func(c *zip.Ctx) {}, "/v1/billing/tier", req, GetTier)

	if w.StatusCode == 500 {
		t.Fatalf("no-org request panicked into a 500; want an explicit refusal")
	}
	if w.StatusCode < 400 {
		t.Fatalf("no-org request answered %d; a tier it could not read must not be served", w.StatusCode)
	}
}
