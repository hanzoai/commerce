package billing

// credit_test.go is the acceptance suite for POST /v1/billing/credit — the ONE
// mint-gated way credit enters an org ledger. It proves the money-critical core:
//   - service token  → grant (the cloud-api path)
//   - global admin   → grant (the human superadmin path)
//   - org admin / plain user / no auth → 403/401, NO grant (a user cannot
//     credit themselves — the whole point of the consolidation)
//   - idempotencyKey → at most ONE grant
//   - starter params (billing/credit constants) compose into a starter grant
//   - the grant is READ by the balance path (spendable immediately)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// postCredit drives POST /v1/billing/credit through the REAL Route() behind seed.
func postCredit(eng *zip.App, tok, orgHdr, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/credit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if orgHdr != "" {
		req.Header.Set("X-Org-Id", orgHdr)
	}
	resp, _ := eng.Test(req)
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var out map[string]any
	raw := respBodyStr(resp)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad json (%d): %v (%s)", resp.StatusCode, err, raw)
	}
	return out
}

// ── the money assertion: only a mint principal may credit ────────────────────

// TestCredit_ServiceTokenGrants proves the legitimate money path: the internal
// service token (cloud-api → commerce) appends a grant (201) end-to-end.
func TestCredit_ServiceTokenGrants(t *testing.T) {
	const tok = "svc-credit-ok"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	resp := postCredit(eng, tok, "creditorg", `{"org":"creditorg","amountCents":500,"reason":"welcome","tag":"starter-credit"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("service-token credit: status=%d body=%s, want 201", resp.StatusCode, respBodyStr(resp))
	}
	out := decodeJSON(t, resp)
	if id, _ := out["id"].(string); id == "" {
		t.Fatalf("credit response missing id: %v", out)
	}
	if amt, _ := out["amountCents"].(float64); int64(amt) != 500 {
		t.Fatalf("amountCents=%v, want 500", out["amountCents"])
	}
}

// TestCredit_GlobalAdminGrants proves the human superadmin (owner=="admin") may
// credit any org named in the body — no X-Org-Id needed; the org is the account.
func TestCredit_GlobalAdminGrants(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", IsAdmin: true}) // global admin
	})
	resp := postCredit(eng, "", "", `{"org":"adminorg","amountCents":1000,"reason":"comp"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("global-admin credit: status=%d body=%s, want 201", resp.StatusCode, respBodyStr(resp))
	}
}

// TestCredit_OrgAdminDenied is THE money assertion: an org-level admin (org owner,
// NOT a global admin, NOT the service token) is 403 — it can never self-credit,
// even though it holds the org Admin bit and passes TokenRequired(Admin).
func TestCredit_OrgAdminDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	eng := engineWithSeed(orgAdminSeed) // owner="acme", IsAdmin=true (org-level)
	resp := postCredit(eng, "", "", `{"org":"acme","amountCents":100000,"reason":"self-credit-attempt"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("org-admin self-credit: status=%d body=%s, want 403 (a user must NOT credit itself)",
			resp.StatusCode, respBodyStr(resp))
	}
}

// TestCredit_PlainUserDenied proves a normal authenticated user (no Admin bit) is
// refused (never a 2xx grant).
func TestCredit_PlainUserDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	eng := engineWithSeed(func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.None))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme"})
	})
	resp := postCredit(eng, "", "acme", `{"org":"acme","amountCents":500,"reason":"x"}`)
	if resp.StatusCode < 400 {
		t.Fatalf("plain user credit: status=%d body=%s, want a rejection (>=400), never a grant",
			resp.StatusCode, respBodyStr(resp))
	}
}

// TestCredit_NoAuthDenied proves an unauthenticated request never grants.
func TestCredit_NoAuthDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	eng := engineWithSeed(nil)
	resp := postCredit(eng, "", "", `{"org":"acme","amountCents":500,"reason":"x"}`)
	if resp.StatusCode < 400 {
		t.Fatalf("no-auth credit: status=%d body=%s, want a rejection (>=400)", resp.StatusCode, respBodyStr(resp))
	}
}

// ── idempotency ──────────────────────────────────────────────────────────────

// TestCredit_IdempotentOnKey proves the same idempotencyKey credits AT MOST ONCE:
// the second call replays the first grant (200, same id) and the balance holds
// exactly one grant.
func TestCredit_IdempotentOnKey(t *testing.T) {
	const tok = "svc-credit-idem"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	body := `{"org":"idemorg","amountCents":250,"reason":"promo","idempotencyKey":"once-2026"}`

	first := postCredit(eng, tok, "idemorg", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first credit: status=%d body=%s, want 201", first.StatusCode, respBodyStr(first))
	}
	firstID, _ := decodeJSON(t, first)["id"].(string)

	second := postCredit(eng, tok, "idemorg", body)
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed credit: status=%d body=%s, want 200 (idempotent replay)", second.StatusCode, respBodyStr(second))
	}
	secondID, _ := decodeJSON(t, second)["id"].(string)
	if firstID == "" || firstID != secondID {
		t.Fatalf("idempotent replay returned a different grant: first=%q second=%q", firstID, secondID)
	}

	// The money invariant: balance == exactly ONE grant, never doubled.
	if bal := getBalanceCents(t, eng, tok, "idemorg"); bal != 250 {
		t.Fatalf("after two same-key credits, balance=%d cents, want exactly 250 (no double-grant)", bal)
	}
}

// ── starter composition + ledger read ────────────────────────────────────────

// TestCredit_StarterComposition proves the starter credit is now just a
// parameterized call driven by the billing/credit constants, and it lands and is
// readable by the balance path (spendable).
func TestCredit_StarterComposition(t *testing.T) {
	const tok = "svc-credit-starter"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	expiry := time.Now().AddDate(0, 0, credit.StarterCreditDays).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"org":"starterorg","amountCents":%d,"reason":"welcome","tag":%q,"expiresAt":%q}`,
		credit.StarterCreditCents, credit.StarterCreditTag, expiry)

	resp := postCredit(eng, tok, "starterorg", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("starter credit: status=%d body=%s, want 201", resp.StatusCode, respBodyStr(resp))
	}
	out := decodeJSON(t, resp)
	if tag, _ := out["tag"].(string); tag != credit.StarterCreditTag {
		t.Fatalf("tag=%q, want %q (domain constant drives it)", out["tag"], credit.StarterCreditTag)
	}

	// Readable by the balance path = spendable.
	if bal := getBalanceCents(t, eng, tok, "starterorg"); bal != int64(credit.StarterCreditCents) {
		t.Fatalf("starter credit not visible in balance: got %d cents, want %d", bal, credit.StarterCreditCents)
	}
}

// TestCredit_LedgerReadableByBalance proves a plain (non-starter) grant nets into
// the org balance the gateway gate reads.
func TestCredit_LedgerReadableByBalance(t *testing.T) {
	const tok = "svc-credit-ledger"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })
	if resp := postCredit(eng, tok, "ledgerorg", `{"org":"ledgerorg","amountCents":700,"reason":"grant"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("credit: status=%d body=%s, want 201", resp.StatusCode, respBodyStr(resp))
	}
	if bal := getBalanceCents(t, eng, tok, "ledgerorg"); bal != 700 {
		t.Fatalf("credit not readable by balance: got %d cents, want 700", bal)
	}
}

// getBalanceCents reads GET /v1/billing/balance for the org pool (subject == org).
func getBalanceCents(t *testing.T, eng *zip.App, tok, orgID string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance?user="+orgID+"&currency=usd", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", orgID)
	resp, terr := eng.Test(req)
	if terr != nil {
		t.Fatalf("balance Test: %v", terr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /balance: status=%d body=%s", resp.StatusCode, respBodyStr(resp))
	}
	bal, _ := decodeJSON(t, resp)["balance"].(float64)
	return int64(bal)
}
