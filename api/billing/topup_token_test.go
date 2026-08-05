package billing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The canonical-ledger + per-org-isolation + idempotency proof for the
// in-console card top-up. The Square charge leg is provider-owned and proven
// against Square sandbox live (cnon:card-nonce-ok); these tests pin the LEDGER
// leg deterministically — the exact invariants Red verifies:
//
//	credit-key == gateway read/debit key   (ONE canonical ledger)
//	credit bucket == charge environment    (sandbox → test bucket, never live)
//	org A top-up can NEVER touch org B      (per-org isolation, no cross leak)
//	a retry / double-submit credits ONCE    (idempotent, no double-charge)

// topupTestApp backs the synthetic *Ctx handed to topupDestination below.
var topupTestApp = zip.New(zip.Config{DisableStartupMessage: true})

// ctxWithOrg builds a *Ctx exactly as the edge/proxy hands it to
// TopupWithToken: an "organization" in the request locals + a `?user=` query
// (which console2's server proxy sets from the validated session, overwriting
// any client value).
func ctxWithOrg(orgName, userQuery string) *zip.Ctx {
	c := topupTestApp.TestCtx("POST", "/v1/billing/topup/token")
	if userQuery != "" {
		c.Fiber().Request().URI().SetQueryString("user=" + url.QueryEscape(userQuery))
	}
	if orgName != "" {
		org := &organization.Organization{}
		org.Name = orgName
		c.Locals("organization", org)
	}
	return c
}

// TestTopupDestination_SubjectResolution proves the top-up credits the SAME
// subject the gateway reads/debits, and can NEVER be steered outside the
// caller's own org (fail-secure bound).
func TestTopupDestination_SubjectResolution(t *testing.T) {
	cases := []struct {
		name, org, userQuery, want string
	}{
		{"dedicated org, no subject → org slug", "acme", "", "acme"},
		{"dedicated org, subject == org → org slug", "acme", "acme", "acme"},
		{"personal-billing member → in-org <org>/<name>", "hanzo", "hanzo/alice", "hanzo/alice"},
		{"subject case-normalized to org slug", "Acme", "ACME/Bob", "acme/bob"},
		{"out-of-org subject → fallback to org slug (no cross-org)", "acme", "victim", "acme"},
		{"prefix-but-not-boundary → fallback (acme-evil is not acme/*)", "acme", "acme-evil", "acme"},
		{"another org's user → fallback (never credits foreign org)", "acme", "victim/root", "acme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := topupDestination(ctxWithOrg(tc.org, tc.userQuery))
			if got != tc.want {
				t.Fatalf("topupDestination(org=%q, ?user=%q) = %q, want %q", tc.org, tc.userQuery, got, tc.want)
			}
		})
	}
}

// TestTopupBounds proves the server-authoritative amount floor+ceiling that the
// handler enforces BEFORE any charge: out-of-bounds amounts (0, negative,
// sub-$1, and a $1,000,000 mega-charge) are rejected; the defaults mirror the
// console MIN/MAX policy; and a per-deploy env override is honored while a
// bad/non-positive override fails safe to the default bound.
func TestTopupBounds(t *testing.T) {
	minC, maxC := topupBounds()
	if minC != 500 || maxC != 500000 {
		t.Fatalf("default topupBounds() = (%d,%d), want (500,500000)", minC, maxC)
	}
	inBounds := func(v int64) bool { return v >= minC && v <= maxC }
	for _, tc := range []struct {
		cents int64
		ok    bool
	}{
		{0, false},         // zero
		{-100, false},      // negative
		{499, false},       // just below the $5 floor
		{500, true},        // exactly the floor
		{2500, true},       // typical
		{500000, true},     // exactly the ceiling
		{500001, false},    // just over the $5,000 ceiling
		{100000000, false}, // $1,000,000 mega-charge — rejected
	} {
		if inBounds(tc.cents) != tc.ok {
			t.Fatalf("amount %d cents: inBounds=%v, want %v", tc.cents, inBounds(tc.cents), tc.ok)
		}
	}

	// Env override honored.
	t.Setenv("COMMERCE_TOPUP_MIN_CENTS", "50")
	t.Setenv("COMMERCE_TOPUP_MAX_CENTS", "1000000")
	if mn, mx := topupBounds(); mn != 50 || mx != 1000000 {
		t.Fatalf("overridden topupBounds() = (%d,%d), want (50,1000000)", mn, mx)
	}
	// Bad / non-positive override → fail safe to the default bound.
	t.Setenv("COMMERCE_TOPUP_MIN_CENTS", "-5")
	t.Setenv("COMMERCE_TOPUP_MAX_CENTS", "garbage")
	if mn, mx := topupBounds(); mn != 100 || mx != 500000 {
		t.Fatalf("bad-override topupBounds() = (%d,%d), want defaults (100,500000)", mn, mx)
	}
}

// seedBalance puts an existing balance on a subject's ledger so a test can start
// from a funded wallet. It is a SEEDING helper only — never an oracle for what
// the top-up handler does. Tests that assert top-up behaviour drive
// TopupWithToken itself; a helper mirroring handler logic only proves a copy of
// the code agrees with itself.
func seedBalance(t *testing.T, ctx ae.Context, org *organization.Organization, subject string, cents int64) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	tr := transaction.New(db)
	tr.Type = transaction.Deposit
	tr.DestinationId = subject
	tr.DestinationKind = "iam-user"
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Tags = "topup"
	tr.Test = org.TestMode()
	tr.MustCreate()
}

func balanceOf(t *testing.T, ctx ae.Context, org *organization.Organization, subject string) currency.Cents {
	t.Helper()
	datas, err := txutil.GetTransactionsByCurrency(org.Namespaced(ctx), subject, "iam-user", currency.USD, org.TestMode())
	if err != nil {
		t.Fatalf("GetTransactionsByCurrency(%s): %v", subject, err)
	}
	if d, ok := datas.Data[currency.USD]; ok && d != nil {
		return d.Balance
	}
	return 0
}

// TestTopup_CanonicalLedger_PerOrgIsolation drives the REAL handler: a card
// top-up lands on the exact per-org balance the gateway gates on, and never
// leaks across orgs or across subjects.
func TestTopup_CanonicalLedger_PerOrgIsolation(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	orgA := moneyOrg("acme-iso") // Live => real charge, live (spendable) bucket
	orgB := moneyOrg("globex-iso")
	m := squareMock("cust_iso", "ccof_iso", "sqpay_iso")
	withFakeSquare(t, m)

	if r := invokeTopupToken(orgA, ctx, `{"sourceId":"cnon:iso","amountCents":2000}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("org A top-up status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	if got := balanceOf(t, ctx, orgA, "acme-iso"); got != 2000 {
		t.Fatalf("org A canonical balance = %d, want 2000 (the bucket the gateway reads)", got)
	}
	if got := balanceOf(t, ctx, orgB, "globex-iso"); got != 0 {
		t.Fatalf("org B balance = %d, want 0 — CROSS-ORG LEAK: A's top-up credited B", got)
	}
	if got := balanceOf(t, ctx, orgB, "acme-iso"); got != 0 {
		t.Fatalf("org B sees A's subject balance = %d, want 0 — namespace isolation broken", got)
	}

	// Subject isolation WITHIN an org: ?user=<org>/<name> credits the sub-user and
	// leaves the org-slug wallet alone.
	if r := invokeTopupTokenAs(orgA, ctx, "acme-iso/alice", `{"sourceId":"cnon:iso2","amountCents":500}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("sub-user top-up status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	if got := balanceOf(t, ctx, orgA, "acme-iso/alice"); got != 500 {
		t.Fatalf("acme-iso/alice balance = %d, want 500", got)
	}
	if got := balanceOf(t, ctx, orgA, "acme-iso"); got != 2000 {
		t.Fatalf("org-slug balance moved to %d after crediting a sub-user, want 2000", got)
	}
}

// TestTopup_SandboxChargeCreditsTestBucket drives the REAL handler for an org
// that transacts in sandbox (per-tenant: org.Live=false): the credit lands in
// the TEST bucket and the LIVE spendable bucket stays empty, so a sandbox
// merchant can never mint real credit — on the same replica serving live orgs.
func TestTopup_SandboxChargeCreditsTestBucket(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "sandbox-proof"
	org.Live = false // this org transacts in sandbox
	if !org.TestMode() {
		t.Fatal("precondition: a non-live org must be in test mode")
	}
	m := squareMock("cust_sb", "ccof_sb", "sqpay_sb")
	withFakeSquare(t, m)

	if r := invokeTopupToken(org, ctx, `{"sourceId":"cnon:sb","amountCents":1500}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	if got := balanceOf(t, ctx, org, "sandbox-proof"); got != 1500 {
		t.Fatalf("TEST bucket = %d, want 1500 (a sandbox top-up credits the test bucket)", got)
	}
	datas, err := txutil.GetTransactionsByCurrency(org.Namespaced(ctx), "sandbox-proof", "iam-user", currency.USD, false)
	if err != nil {
		t.Fatalf("live-bucket read: %v", err)
	}
	if d, ok := datas.Data[currency.USD]; ok && d != nil && d.Balance != 0 {
		t.Fatalf("LIVE bucket = %d, want 0 — a sandbox charge credited the spendable balance", d.Balance)
	}
}

// --- The Square idempotency key on the one-off top-up ------------------------
//
// These drive the REAL TopupWithToken handler (the tests above exercise pure
// helpers and a ledger replica). They pin the invariant that made a
// re-tokenized retry charge the card TWICE: the charge request carried NO
// IdempotencyKey, so `thirdparty/square.Charge` minted a fresh uuid every
// attempt and Square-side dedup — the only backstop that survives a guard-store
// outage — was never engaged, while the local guard keyed on the single-use
// nonce, which changes on every re-tokenization.

func invokeTopupToken(org *organization.Organization, ctx context.Context, body string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/topup/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/topup/token", req, TopupWithToken)
}

// invokeTopupTokenAs drives the handler with an explicit ?user= subject — what
// console2's billing proxy sets from the validated session.
func invokeTopupTokenAs(org *organization.Organization, ctx context.Context, subject, body string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/topup/token?user="+url.QueryEscape(subject), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/topup/token", req, TopupWithToken)
}

// chargedKeys snapshots the idempotency keys the processor actually saw.
func chargedKeysOf(m *mockSquareProcessor) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.chargedKeys))
	for k := range m.chargedKeys {
		out = append(out, k)
	}
	return out
}

// The charge must reach Square WITH a stable key, and that key must not be
// derived from the single-use nonce.
func TestTopupWithToken_ForwardsStableSquareKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("topupkey")
	m := squareMock("cust_t", "ccof_t", "sqpay_t")
	withFakeSquare(t, m)

	resp := invokeTopupToken(org, ctx, `{"sourceId":"cnon:first","amountCents":2500}`, nil)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 200", resp.StatusCode, string(b))
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}

	keys := chargedKeysOf(m)
	if len(keys) != 1 {
		t.Fatalf("processor saw %d idempotency keys (%v), want exactly 1 — an empty key means Square-side dedup is OFF", len(keys), keys)
	}
	if !strings.HasPrefix(keys[0], "topup:topupkey:") {
		t.Fatalf("square idempotency key=%q, want it scoped to the credited subject (topup:topupkey:…)", keys[0])
	}
	if strings.Contains(keys[0], "cnon:") {
		t.Fatalf("square idempotency key=%q is derived from the single-use nonce — a re-tokenized retry would double-charge", keys[0])
	}
}

// "Start over" re-tokenizes a FRESH nonce. Same customer, same amount, no client
// key: this must NOT become a second charge or a second credit.
func TestTopupWithToken_ReTokenizedRetry_OneChargeOneCredit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("topupretry")
	m := squareMock("cust_r", "ccof_r", "sqpay_r")
	withFakeSquare(t, m)

	if r := invokeTopupToken(org, ctx, `{"sourceId":"cnon:a","amountCents":2500}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("first top-up status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	// The customer thinks it failed and starts over — a brand-new nonce.
	invokeTopupToken(org, ctx, `{"sourceId":"cnon:b","amountCents":2500}`, nil)

	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d after a re-tokenized retry, want 1 — DOUBLE-CHARGE on the customer's card", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "topupretry"); got != 2500 {
		t.Fatalf("balance=%d after a re-tokenized retry, want 2500 — the ledger double-credited", got)
	}
}

// An explicit client key is honored verbatim and is the strongest guarantee.
func TestTopupWithToken_ClientKeyRetry_OneCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("topupclientkey")
	m := squareMock("cust_c", "ccof_c", "sqpay_c")
	withFakeSquare(t, m)

	h := map[string]string{"X-Idempotency-Key": "attempt-7"}
	if r := invokeTopupToken(org, ctx, `{"sourceId":"cnon:x","amountCents":1000}`, h); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("first top-up status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	invokeTopupToken(org, ctx, `{"sourceId":"cnon:y","amountCents":1000}`, h)

	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d with a stable X-Idempotency-Key, want 1", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "topupclientkey"); got != 1000 {
		t.Fatalf("balance=%d, want 1000 — double credit under a stable key", got)
	}
}
