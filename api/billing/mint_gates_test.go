package billing

// mint_gates_test.go proves the three C1-reopened mint surfaces are closed:
//   - ZAP billing.deposit  (the /zap dispatch mint method)
//   - /allotment/grant     (the subscription-clamped plan)
//   - /me/welcome          (the TOCTOU double-mint)
// plus that each legitimate (service-token / privileged) path still mints.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hanzoai/commerce/models/organization"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// orgAdminSeed sets the exact C1 adversary: an org-level admin (org-level
// isAdmin, NOT a SuperAdmin) — a gateway-minted org owner or legacy per-org
// Admin token. NOT the internal service token, NOT a platform global admin.
func orgAdminSeed(c *zip.Ctx) {
	c.Locals("iam_authenticated", true)
	c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
	c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
}

// ── ZAP billing.deposit ─────────────────────────────────────────────────────

// TestZapDeposit_OrgAdminDenied is THE acceptance test for the [CRITICAL] miss:
// an org admin POSTing the ZAP billing.deposit method is 403 — it can no longer
// mint iam-user balance from a client amount via /zap (while /deposit already
// 403'd it). Reproduces RED's exact vector (amount 100000000, no ceiling).
func respBodyStr(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func TestZapDeposit_OrgAdminDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	eng := engineWithSeed(orgAdminSeed)

	body := `{"method":"billing.deposit","params":{"user":"acme","amount":100000000,"currency":"usd"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/zap", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("org-admin /zap billing.deposit: status=%d body=%s, want 403", resp.StatusCode, respBodyStr(resp))
	}
}

// TestZapDeposit_ServiceTokenMints proves the legitimate path still mints: the
// internal service token reaches zapDeposit and creates the deposit (200 with a
// result). cloud-api authenticates exactly this way.
func TestZapDeposit_ServiceTokenMints(t *testing.T) {
	const tok = "svc-zap-mint"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	body := `{"method":"billing.deposit","id":"z1","params":{"user":"zapmintorg","amount":250,"currency":"usd"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/zap", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "zapmintorg")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("service-token /zap billing.deposit: status=%d body=%s, want 200", resp.StatusCode, respBodyStr(resp))
	}
	var zapResp struct {
		Result map[string]any            `json:"result"`
		Error  *struct{ Message string } `json:"error"`
	}
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &zapResp); err != nil {
		t.Fatalf("bad zap response: %v (%s)", err, raw)
	}
	if zapResp.Error != nil {
		t.Fatalf("service-token deposit returned zap error: %s", zapResp.Error.Message)
	}
	if tid, _ := zapResp.Result["transactionId"].(string); tid == "" {
		t.Fatalf("service-token deposit missing transactionId: %v", zapResp.Result)
	}
}

// TestZapReads_OrgAdminNotBlocked proves the gate does NOT over-block: an org
// admin can still call the ZAP read method (billing.getBalance) — only the mint
// method is gated.
func TestZapReads_OrgAdminNotBlocked(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "acme"
	org.Live = true
	eng := engineWithSeed(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", org)
		orgAdminSeed(c)
	})

	body := `{"method":"billing.getBalance","id":"r1","params":{"user":"acme/alice","currency":"usd"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/zap", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("org-admin /zap billing.getBalance was 403 — the mint gate over-blocked a read")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("org-admin /zap billing.getBalance: status=%d body=%s, want 200", resp.StatusCode, respBodyStr(resp))
	}
}

// ── /allotment/grant plan clamp ─────────────────────────────────────────────

// TestAllotment_OrgAdminCannotInflatePlan is the acceptance test for the [HIGH]
// miss: an org admin naming the paid "max" plan (which grants $100/mo) for a
// subject with NO matching subscription is clamped to their real entitlement
// (0 cents) — they cannot mint the higher tier's allotment for free.
func TestAllotment_OrgAdminCannotInflatePlan(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "acme"
	org.Live = true
	eng := engineWithSeed(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", org)
		orgAdminSeed(c)
	})

	body := `{"user":"acme","plan":"max"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/allotment/grant", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	// The route is deliberately NOT platform-only (an org grants its OWN
	// allotment); the defense is the clamp, so it must be reachable, not 403.
	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("allotment/grant should be reachable for the org (clamped), got 403")
	}
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad response: %v (%s)", err, raw)
	}
	if amt, _ := out["amountCents"].(float64); amt != 0 {
		t.Fatalf("org-admin named 'max' without subscription: amountCents=%v, want 0 (clamped to real entitlement)", amt)
	}
}

// TestAllotment_OrgAdminMatchingSubscriptionHonored proves the clamp is not
// over-strict: an org admin granting the allotment for a subject who REALLY IS
// subscribed to "max" — via a PAYMENT-BACKED subscription — gets the max plan's
// declared credit (10000 cents). This is RED's intended behavior — "an org
// legitimately grants its own allotment, it just must match the paid plan" — and
// validates that subscriptionPlanSlug actually finds the subscription (the
// Ancestor(synckey) fix). C1-a: the anchor MUST be payment-backed (external
// provider or invoiced), never a zero-payment internal Active sub — proven by
// TestSubscriptionPlanSlug_PaymentBackedOnly + TestPlanForGrant_OrgAdminCannotAnchorOnForgedSub.
func TestAllotment_OrgAdminMatchingSubscriptionHonored(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	ctx := ae.NewContext()
	defer ctx.Close()

	const ns = "submatch"
	org := &organization.Organization{}
	org.Name = ns
	org.Live = true

	// The subject really is subscribed to "max", and it is PAYMENT-BACKED (an
	// external provider manages billing) — the only kind of paid-tier sub whose
	// allotment may be minted. Seed it in the org's namespace.
	sub := subscription.New(datastore.New(nscontext.WithNamespace(ctx, ns)))
	sub.UserId = ns
	sub.Plan.Slug = "max"
	sub.Status = subscription.Active
	sub.ProviderType = "stripe"
	sub.PeriodStart = time.Now()
	if err := sub.Create(); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}

	eng := engineWithSeed(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", org)
		orgAdminSeed(c)
	})
	body := `{"user":"` + ns + `","plan":"max"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/allotment/grant", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("allotment/grant for a real subscriber should be reachable, got 403")
	}
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad response: %v (%s)", err, raw)
	}
	if amt, _ := out["amountCents"].(float64); amt != 10000 {
		t.Fatalf("org-admin with a real 'max' subscription: amountCents=%v, want 10000 (clamp honors the matching paid plan)", amt)
	}
}

// TestAllotment_ServiceTokenMayNameAnyPlan proves the privileged override is
// preserved: the internal service token (comps/backfills) may name "max" and
// mint its declared $100/mo (10000 cents).
func TestAllotment_ServiceTokenMayNameAnyPlan(t *testing.T) {
	const tok = "svc-allot"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	body := `{"user":"allotorg","plan":"max"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/allotment/grant", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "allotorg")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("service-token allotment/grant: status=%d body=%s, want 200/201", resp.StatusCode, respBodyStr(resp))
	}
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad response: %v (%s)", err, raw)
	}
	if amt, _ := out["amountCents"].(float64); amt != 10000 {
		t.Fatalf("service-token named 'max': amountCents=%v, want 10000 (privileged override honored)", amt)
	}
}

// ── /me/welcome TOCTOU ──────────────────────────────────────────────────────

// TestWelcome_ConcurrentGrantsCreditOnce proves the [LOW] TOCTOU is closed: N
// concurrent first-time welcome grants credit the subject AT MOST ONCE. The
// deterministic key collapses concurrent creates onto one row (ON CONFLICT), so
// the final balance is exactly one $100 credit, never 2×.
func TestWelcome_ConcurrentGrantsCreditOnce(t *testing.T) {
	const tok = "svc-welcome"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })

	// The welcome credit now requires a chargeable card on file (it is real money;
	// without the card, open signup turns it into a faucet). Seed one for the
	// subject so this test still exercises the TOCTOU invariant it is about.
	seedCardOnFile(t, eng, tok, "welcomeorg")

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/billing/me/welcome", bytes.NewBufferString(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("X-Org-Id", "welcomeorg")
			_, _ = eng.Fiber().Test(req)
		}()
	}
	wg.Wait()

	// The money invariant: balance == exactly one starter credit, never doubled.
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/me/balance?currency=usd", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "welcomeorg")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me/balance: status=%d body=%s", resp.StatusCode, respBodyStr(resp))
	}
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad balance response: %v (%s)", err, raw)
	}
	balance, _ := out["balance"].(float64)
	if int64(balance) != credit.StarterCreditCents {
		t.Fatalf("after %d concurrent welcomes, balance=%d cents, want exactly %d (one credit, no double-mint)",
			n, int64(balance), credit.StarterCreditCents)
	}
}

// seedCardOnFile vaults a chargeable card through the SAME HTTP surface the
// welcome handler reads, so the card lands on whatever subject/namespace the
// middleware resolves — no guessing at org namespacing in the fixture.
func seedCardOnFile(t *testing.T, eng *zip.App, tok, org string) {
	t.Helper()
	body := fmt.Sprintf(`{"customerId":%q,"type":"card","isDefault":true,"providerRef":"sq-card-test","card":{"brand":"visa","last4":"4242","expMonth":12,"expYear":2032}}`, org)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/payment-methods", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", org)
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed payment method: status=%d body=%s", resp.StatusCode, respBodyStr(resp))
	}
}

// TestWelcome_RequiresPaymentMethod locks the offering: no card on file => no
// starter credit (200 granted=false), so a bot farm cannot mint accounts and
// drain the credit pool. Free tier = run the OSS bot with your own keys.
func TestWelcome_RequiresPaymentMethod(t *testing.T) {
	const tok = "svc-welcome-nopm"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/me/welcome", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "nocardorg")
	resp, terr := eng.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("welcome without a card: status=%d body=%s", resp.StatusCode, respBodyStr(resp))
	}
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad welcome response: %v (%s)", err, raw)
	}
	if granted, _ := out["granted"].(bool); granted {
		t.Fatal("welcome credit granted with NO payment method on file — the credit is a faucet")
	}
	if reason, _ := out["reason"].(string); reason != "payment_method_required" {
		t.Fatalf("reason = %q, want payment_method_required", reason)
	}
}
