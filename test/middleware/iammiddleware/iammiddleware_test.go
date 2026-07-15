// Copyright © 2026 Hanzo AI. MIT License.
//
// Gateway-trust shim tests. The legacy in-binary JWT validation tests
// (~600 LOC of RSA keys + JWKS server + claim shaping) were removed
// when the trust boundary moved to hanzoai/gateway. What's left is a
// minimal contract: when X-Org-Id is present, IAMTokenRequired
// resolves the org and sets the legacy context locals; when it's absent, it
// falls through to legacy auth.
//
// NB: the earlier `var _ = Describe(...)` Ginkgo block was dead — the JWKS/
// claim helpers (newClient/signToken/makeAdminClaims) and the ginkgo dot-import
// it depended on were deleted in the same migration (see suite_test.go), so it
// never compiled. Removed so the package builds, vets, and runs the stdlib
// contract test below. Coverage of the IAM claim→permission mapping now lives in
// middleware/iammiddleware's own package tests.

package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
)

func TestFallthroughWithoutHeaders(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(iammiddleware.IAMTokenRequired())
	app.Get("/x", func(c *zip.Ctx) error { return c.String(http.StatusOK, "ok") })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
}
