package billing

// money_guard_test.go covers the idempotency guard on the two SAVED-CARD money
// moves and the fail-closed branch on both card paths. Every test here drives a
// REAL handler — TopupWithToken, Topup, RunAutoRechargeAllOrgs — through the
// same seams production uses (processorsForOrg for the gateway, idemBegin for
// the guard store). Nothing re-implements handler logic.
//
// The holes these pin:
//
//	guard store down  → the charge is REFUSED, not waved through
//	cron fires twice  → the customer's card is charged ONCE
//	cron misconfigured→ an out-of-bounds amount never reaches the card
//	body userId       → cannot steer the credit off the caller's own org

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// breakGuardStore makes the idempotency guard unavailable for the duration of
// the test — the "we cannot tell a first attempt from a retry" state.
func breakGuardStore(t *testing.T) {
	t.Helper()
	old := idemBegin
	idemBegin = func(*datastore.Datastore, string, string) (*idempotencykey.IdempotencyKey, bool, error) {
		return nil, false, errors.New("guard store unavailable")
	}
	t.Cleanup(func() { idemBegin = old })
}

// A guard-store outage must REFUSE the one-off top-up. Failing open here used to
// be justified by "the single-use nonce is still a backstop" — but a
// re-tokenized retry carries a fresh nonce, so in exactly this branch there is
// no backstop at all.
func TestTopupToken_GuardStoreDown_RefusesWithoutCharging(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("guarddown-token")
	m := squareMock("cust_g", "ccof_g", "sqpay_g")
	withFakeSquare(t, m)
	breakGuardStore(t)

	resp := invokeTopupToken(org, ctx, `{"sourceId":"cnon:ok","amountCents":2500}`, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 503 — an unusable guard must refuse, not charge blind", resp.StatusCode, string(b))
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d with the guard store down, want 0 — the card was charged with no replay protection", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "guarddown-token"); got != 0 {
		t.Fatalf("balance=%d, want 0 — a refused top-up must not credit", got)
	}
}

// ─── saved-card top-up + auto-recharge (chargeAndCredit) ────────────────────

// seedSavedCard persists a default card-on-file for subject, the payment method
// the saved-card top-up and the auto-recharge cron both charge.
func seedSavedCard(t *testing.T, db *datastore.Datastore, subject, cardID, customerID string) *paymentmethod.PaymentMethod {
	t.Helper()
	pm := paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = cardID
	pm.ProviderType = string(processor.Square)
	pm.IsDefault = true
	pm.Metadata = map[string]interface{}{"squareCustomerId": customerID, "squareCardId": cardID}
	if err := pm.Create(); err != nil {
		t.Fatalf("seed payment method: %v", err)
	}
	return pm
}

func invokeTopup(org *organization.Organization, ctx context.Context, body string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/topup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/topup", req, Topup)
}

// The saved-card path has NO nonce backstop at all — a card-on-file id is
// reusable — so a guard-store outage must refuse.
func TestTopup_GuardStoreDown_RefusesWithoutCharging(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("guarddown-saved")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, "guarddown-saved", "ccof_s", "cust_s")

	m := squareMock("", "", "sqpay_s")
	withFakeSquare(t, m)
	breakGuardStore(t)

	body := fmt.Sprintf(`{"userId":"guarddown-saved","paymentMethodId":%q,"amountCents":2500}`, pm.Id())
	resp := invokeTopup(org, ctx, body, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 503", resp.StatusCode, string(b))
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d with the guard store down, want 0 — a REUSABLE saved card was charged with no replay protection", m.chargeCalls)
	}
}

// A body userId must never steer the credit outside the caller's own org. The
// handler narrows it by the same in-org rule the token and subscribe paths use.
func TestTopup_ForeignSubject_CreditsOwnOrgOnly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("narrow-org")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, "narrow-org", "ccof_n", "cust_n")

	m := squareMock("", "", "sqpay_n")
	withFakeSquare(t, m)

	// A hostile body names someone else's account as the destination.
	body := fmt.Sprintf(`{"userId":"victim","paymentMethodId":%q,"amountCents":2500}`, pm.Id())
	if r := invokeTopup(org, ctx, body, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status=%d body=%s, want 200", r.StatusCode, string(b))
	}

	// The money landed on the caller's OWN org slug, never on "victim".
	if got := balanceOf(t, ctx, org, "narrow-org"); got != 2500 {
		t.Fatalf("own-org balance=%d, want 2500 — the credit did not land on the caller's org", got)
	}
	if got := balanceOf(t, ctx, org, "victim"); got != 0 {
		t.Fatalf("foreign subject balance=%d, want 0 — a body userId steered the credit off the caller's org", got)
	}
}

// ─── the auto-recharge cron ─────────────────────────────────────────────────

// seedRechargeOrg persists an organization plus an enabled auto-recharge config
// and a default saved card — everything RunAutoRechargeAllOrgs needs to find it.
func seedRechargeOrg(t *testing.T, ctx ae.Context, name string, amountCents int64) *organization.Organization {
	t.Helper()
	org := organization.New(datastore.New(ctx))
	org.Name = name
	org.Live = true
	if err := org.Create(); err != nil {
		t.Fatalf("create org: %v", err)
	}

	db := datastore.New(org.Namespaced(ctx))
	seedSavedCard(t, db, name, "ccof_"+name, "cust_"+name)

	cfg := autorecharge.New(db)
	cfg.UserId = name
	cfg.Enabled = true
	cfg.ThresholdCents = 1000 // a fresh org sits at 0, so the cron fires
	cfg.AmountCents = amountCents
	cfg.Currency = "usd"
	if err := cfg.Create(); err != nil {
		t.Fatalf("create autorecharge config: %v", err)
	}
	return org
}

func runRecharge(ctx context.Context) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/auto-recharge/run-all", nil)
	return driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
	}, "/v1/billing/auto-recharge/run-all", req, RunAutoRechargeAllOrgs)
}

// THE off-session hole: a cron that re-fires (overlapping run, retried job)
// charging a reusable saved card a second time, with nobody watching and the
// customer finding it on their statement. The recharge key is derived from the
// recharge's own identity in a window, so the second fire replays instead.
func TestAutoRecharge_DoubleFire_OneCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "cron-once", 2500)

	m := squareMock("", "", "sqpay_cron")
	withFakeSquare(t, m)

	if r := runRecharge(ctx); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("first run status=%d body=%s, want 200", r.StatusCode, string(b))
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls after first run=%d, want 1", m.chargeCalls)
	}

	// The job fires again inside the same window — a retry, an overlapping
	// schedule, a pod restart mid-run. The card must NOT be charged twice.
	if r := runRecharge(ctx); r.StatusCode != http.StatusOK {
		t.Fatalf("second run status=%d, want 200", r.StatusCode)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls after a re-fired cron=%d, want 1 — DOUBLE-CHARGE on a real customer card, off-session", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "cron-once"); got != 2500 {
		t.Fatalf("balance=%d after a re-fired cron, want 2500 — the ledger double-credited", got)
	}
}

// A mis-set config must not charge an arbitrary amount off-session. The bound is
// the same [min,max] the token top-up enforces, applied in the shared primitive
// so both callers inherit it.
func TestAutoRecharge_AmountAboveCap_NeverCharges(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "cron-cap", 100000000) // $1,000,000

	m := squareMock("", "", "sqpay_cap")
	withFakeSquare(t, m)

	if r := runRecharge(ctx); r.StatusCode != http.StatusOK {
		t.Fatalf("run status=%d, want 200 (a skipped org is a handled outcome)", r.StatusCode)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d for a $1,000,000 auto-recharge, want 0 — an out-of-bounds amount reached a real card", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "cron-cap"); got != 0 {
		t.Fatalf("balance=%d, want 0", got)
	}
}
