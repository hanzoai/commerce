// Copyright © 2026 Hanzo AI. MIT License.
//
// Gateway-trust shim tests. The legacy in-binary JWT validation tests
// (~600 LOC of RSA keys + JWKS server + claim shaping) were removed
// when the trust boundary moved to hanzoai/gateway. What's left is a
// minimal contract: when X-Org-Id is present, IAMTokenRequired
// resolves the org and sets the legacy gin keys; when it's absent, it
// falls through to legacy auth.

package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	pkgAuth "github.com/hanzoai/commerce/pkg/auth"
)

func TestFallthroughWithoutHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(iammiddleware.IAMTokenRequired())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%q", w.Code, w.Body.String())
	}
}

func TestIsIAMAuthenticatedDefaultsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := &gin.Context{Request: httptest.NewRequest(http.MethodGet, "/x", nil)}
	if iammiddleware.IsIAMAuthenticated(c) {
		t.Fatalf("expected false on bare gin.Context")
	}
}

func TestIsIAMAuthenticatedWithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(pkgAuth.HeaderOrgID, "hanzo")
	c := &gin.Context{Request: req}
	if !iammiddleware.IsIAMAuthenticated(c) {
		t.Fatalf("expected true when X-Org-Id is present")
	}
}

func TestGetIAMClaimsAlwaysNil(t *testing.T) {
	// Public-API contract: GetIAMClaims is retained for source compat but
	// returns nil under gateway-trust. Call sites that need user info
	// must read X-User-Id / X-User-Email via pkg/auth.
	if iammiddleware.GetIAMClaims(nil) != nil {
		t.Fatalf("GetIAMClaims must return nil under gateway-trust")
	}
}

func TestGetIAMTierAlwaysEmpty(t *testing.T) {
	if got := iammiddleware.GetIAMTier(nil); got != "" {
		t.Fatalf(`GetIAMTier must return "" under gateway-trust, got %q`, got)
	}
}

// Make the test types line up with the legacy organization model so
// future contract tests can extend without import-cycle gymnastics.
var _ = organization.Organization{}
