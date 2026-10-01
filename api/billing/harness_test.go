package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/organization"
)

// invokeMoneyHandler drives a single billing handler directly with a real
// org + datastore context and an optional header set — the harness for the H1
// money-correctness tests (Deposit behavior, independent of the C1 gate, which is
// proven separately). The org and ctx are shared with the test's setup
// so the handler's org.Namespaced(c.Context()) resolves the SAME namespace + defaultDB the
// setup wrote to.
func invokeMoneyHandler(org *organization.Organization, ctx context.Context, h zip.Handler, body string, headers map[string]string) *http.Response {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	// These money handlers (Deposit) run in production ONLY behind
	// middleware.PlatformOnly, which calls AuthorizeMint. Invoking them directly
	// here skips that middleware, so authorize the datastore context to faithfully
	// reproduce the post-gate state; the GATE itself (org-admin → 403 / ledger-sink
	// refusal) is proven separately in mint_surface_test.go and the C1 tests.
	app.Raw(http.MethodPost, "/v1/billing/x", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(mintauth.WithAuthorized(ctx))
		return c.Next()
	}, h)
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	return resp
}

// driveSeeded mounts handler on a throwaway zip app behind seed (which sets the
// locals a live middleware chain would set), drives req through REAL routing so
// path params, query, and body parse exactly as production, and returns the
// recorder. routePattern carries the :param placeholders; req's URL is the
// concrete path. The shared harness for the direct-handler-drive tests that a
// gin CreateTestContext + handler(c) used to express.
func driveSeeded(seed func(*zip.Ctx), routePattern string, req *http.Request, handler zip.Handler) *http.Response {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(zip.MethodAll, routePattern, func(c *zip.Ctx) error {
		if seed != nil {
			seed(c)
		}
		return c.Next()
	}, handler)
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func txID(t *testing.T, r *http.Response) string {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad response json: %v (%s)", err, string(raw))
	}
	id, _ := out["transactionId"].(string)
	return id
}

func moneyOrg(name string) *organization.Organization {
	o := &organization.Organization{}
	o.Name = name
	o.Live = true
	return o
}
