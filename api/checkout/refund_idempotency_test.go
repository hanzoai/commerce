package checkout

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// refundApp registers the real Refund handler on a fresh zip app. The route
// handler first seeds the locals a live request's auth/identity + request
// context middleware would set — the caller's organization, the namespaced
// request context, and the verified admin claim the gateway/EdgeAuth mints (so
// middleware.RequireAdmin authorizes the money move) — then calls Refund on the
// SAME ctx. Driving it through app.Fiber().Test exercises the real route so
// fiber populates the :orderid param exactly as production does.
func refundApp(org *organization.Organization, base context.Context, ns string) *zip.App {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/v1/order/:orderid/refund", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(base)
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns, IsAdmin: true})
		return Refund(c)
	})
	return app
}

// TestRefund_Idempotency_ReplayReturnsStored proves the refund guard: once a
// refund with key K has completed, a SECOND POST with the same K returns the
// stored response and NEVER re-enters refund() (which would hit the gateway and
// double-refund). We assert this by pre-seeding a completed guard for the order
// and checking the replay returns exactly the stored body — no Square creds,
// no second money move.
func TestRefund_Idempotency_ReplayReturnsStored(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const ns = "acme"
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)

	// Seed an order in the org namespace so getOrganizationAndOrder resolves it.
	ord := order.New(db)
	ord.Total = 5000
	ord.Paid = 5000
	if err := ord.Create(); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	// Pre-record a COMPLETED refund guard for this order + key, with a sentinel
	// response body — as if a first refund already succeeded.
	scope := "refund:" + ord.Id()
	rec, replay, err := idempotencykey.Begin(db, scope, "refund_key_1")
	if err != nil || replay {
		t.Fatalf("seed guard begin: err=%v replay=%v", err, replay)
	}
	sentinel := `{"id":"` + ord.Id() + `","refunded":1500,"sentinel":true}`
	if err := idempotencykey.Complete(rec, sentinel); err != nil {
		t.Fatalf("seed guard complete: %v", err)
	}

	// Now drive the Refund handler with the SAME key. It must short-circuit to
	// the stored response, returning 200 + the sentinel body verbatim.
	org := &organization.Organization{}
	org.Name = ns

	app := refundApp(org, base, ns)
	body, _ := json.Marshal(map[string]any{"amount": 1500})
	req := httptest.NewRequest(http.MethodPost, "/v1/order/"+ord.Id()+"/refund", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "refund_key_1")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("refund request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("replay status = %d, want 200; body=%s", resp.StatusCode, string(respBody))
	}
	var got map[string]any
	if err := json.Unmarshal(respBody, &got); err != nil {
		t.Fatalf("replay body not JSON: %s", string(respBody))
	}
	if got["sentinel"] != true {
		t.Fatalf("replay did not return the stored response (would have re-refunded!); body=%s", string(respBody))
	}
}

// TestRefund_Idempotency_InFlightRejected proves that while a refund with key K
// is in-flight (guard started, not completed), a concurrent second request with
// the same key is rejected with 409 rather than allowed to double-refund.
func TestRefund_Idempotency_InFlightRejected(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const ns = "acme"
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)

	ord := order.New(db)
	ord.Total = 5000
	ord.Paid = 5000
	if err := ord.Create(); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	// Start (but do NOT complete) a guard — simulating an in-flight refund.
	scope := "refund:" + ord.Id()
	if _, replay, err := idempotencykey.Begin(db, scope, "inflight_key"); err != nil || replay {
		t.Fatalf("seed in-flight guard: err=%v replay=%v", err, replay)
	}

	org := &organization.Organization{}
	org.Name = ns

	app := refundApp(org, base, ns)
	body, _ := json.Marshal(map[string]any{"amount": 1500})
	req := httptest.NewRequest(http.MethodPost, "/v1/order/"+ord.Id()+"/refund", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "inflight_key")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("refund request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 409 {
		t.Fatalf("in-flight replay status = %d, want 409 (must not double-refund); body=%s", resp.StatusCode, string(respBody))
	}
}
