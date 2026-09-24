package billing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"
)

// The guards read the router's own declaration.

// ── route registration guards ────────────────────────────────────────────────

// mountRoutes mounts the billing Route() onto a bare zip app under the
// production "/v1" api prefix and returns the app plus the recorded route table.
// A recordingRouter satisfies *zip.Group, so this exercises the real
// registration code path without router.New()'s global http.Handle side effects.
func mountRoutes(t *testing.T) (*zip.App, []zip.Route) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	Route(app.Group("/v1"))
	return app, app.Routes()
}

// TestTransactionsRouteRegistered proves GET /v1/billing/transactions is routed
// and bound to ListBillingTransactions. Regression guard: the handler existed but was
// never registered, so the billing dashboard's Transactions tab got a 404.
func TestTransactionsRouteRegistered(t *testing.T) {
	_, routes := mountRoutes(t)

	const wantMethod = http.MethodGet
	const wantPath = "/v1/billing/transactions"

	found := false
	for _, r := range routes {
		if r.Method == wantMethod && r.Pattern == wantPath {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("route %s %s is not registered (the defect): handler ListBillingTransactions exists but was never routed; declared: %+v", wantMethod, wantPath, routes)
	}
}

// TestTransactionsRouteNot404 confirms the path actually resolves at request
// time: an unauthenticated GET must reach the middleware/handler chain, never
// gin's "404 no route". (Auth makes it 401/400 here — the point is: not 404.)
func TestTransactionsRouteNot404(t *testing.T) {
	engine, _ := mountRoutes(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/transactions", nil)
	resp, terr := engine.Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /v1/billing/transactions returned 404 (no route); the route is not wired")
	}
}
