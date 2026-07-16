package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestGrantRoutesConsolidated_BespokeRoutesGone proves the consolidation to ONE
// grant primitive: the three bespoke grant routes no longer exist. Comped credit
// is minted ONLY via POST /v1/billing/credit-grants (mint-gated). The `billing`
// group carries prefix auth (.Use), so an UNAUTHENTICATED call to any path — even
// a removed one — 401s at the middleware before routing. The unambiguous
// "route removed" signal is therefore: a fully-AUTHENTICATED caller (a valid
// service token that clears the prefix gate) gets 404 (no such route).
func TestGrantRoutesConsolidated_BespokeRoutesGone(t *testing.T) {
	const tok = "svc-consolidation"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	eng := engineWithSeed(nil)

	for _, path := range []string{
		"/v1/billing/me/welcome",    // was PostMyWelcome     (self-service welcome)
		"/v1/billing/credit",        // was GrantStarterCredit (self-service starter)
		"/v1/billing/grant-starter", // was GrantStarter       (service starter twin)
	} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"user":"acme/alice"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("X-Org-Id", "acme")
		resp, err := eng.Fiber().Test(req)
		if err != nil {
			t.Fatalf("Test %s: %v", path, err)
		}
		// An authenticated caller must get 404 (route gone), never a working grant.
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			t.Errorf("POST %s: status=%d — this bespoke grant route still GRANTS; it must be removed (credit only via /credit-grants)", path, resp.StatusCode)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("POST %s: status=%d, want 404 — the bespoke grant route must be REMOVED", path, resp.StatusCode)
		}
	}
}

// TestCreditGrant_ServiceTokenIdempotent proves the ONE grant primitive is
// exactly-once under a client X-Idempotency-Key: two identical submits create a
// SINGLE grant (the second replays the first's stored body), so a composing
// program — the ai/cloud "$5 starter" trigger, the admin cockpit, the CLI — can
// retry safely with no double credit. It also shows the "$5 starter" IS just this
// endpoint composed with the shared billing/credit constants (amount + tag).
func TestCreditGrant_ServiceTokenIdempotent(t *testing.T) {
	const tok = "svc-credit-grant"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	eng := engineWithSeed(func(c *zip.Ctx) { c.SetContext(ctx) })

	body := fmt.Sprintf(
		`{"userId":"cgorg/alice","amountCents":%d,"currency":"usd","name":"Welcome credit","tags":%q,"expiresIn":"8760h"}`,
		credit.StarterCreditCents, credit.StarterCreditTag,
	)

	post := func() (*http.Response, map[string]any) {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/credit-grants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("X-Org-Id", "cgorg")
		req.Header.Set("X-Idempotency-Key", "starter:cgorg/alice")
		resp, err := eng.Fiber().Test(req)
		if err != nil {
			t.Fatalf("Test: %v", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return resp, out
	}

	resp1, out1 := post()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("first grant: status=%d, want 201", resp1.StatusCode)
	}
	id1, _ := out1["id"].(string)
	if id1 == "" {
		t.Fatalf("first grant missing id: %v", out1)
	}
	if amt, _ := out1["amountCents"].(float64); int64(amt) != credit.StarterCreditCents {
		t.Fatalf("amountCents=%v, want %d (the $5 starter composed via credit-grants)", out1["amountCents"], credit.StarterCreditCents)
	}
	if tag, _ := out1["tags"].(string); tag != credit.StarterCreditTag {
		t.Fatalf("tags=%q, want %q", tag, credit.StarterCreditTag)
	}

	resp2, out2 := post()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("replayed grant: status=%d, want 200 (idempotent replay, not a second 201)", resp2.StatusCode)
	}
	if id2, _ := out2["id"].(string); id2 != id1 {
		t.Fatalf("replayed grant id=%q, want %q — a retried grant must be the SAME grant, never a second one", id2, id1)
	}
}
