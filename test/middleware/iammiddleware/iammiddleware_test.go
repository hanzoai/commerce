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

	"github.com/hanzoai/commerce/auth"
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

func TestGetIAMClaimsNilContextReturnsZeroClaims(t *testing.T) {
	// Public-API contract: GetIAMClaims is always non-nil under
	// gateway-trust. A nil gin.Context (test ergonomics) yields a
	// zero-valued *auth.IAMClaims so call sites can read fields without
	// guarding.
	got := iammiddleware.GetIAMClaims(nil)
	if got == nil {
		t.Fatalf("GetIAMClaims(nil) must return non-nil claims, got nil")
	}
	if got.IsAdmin || got.Owner != "" || got.Subject != "" {
		t.Fatalf("GetIAMClaims(nil) must be zero-valued, got %+v", got)
	}
}

func TestGetIAMClaimsFromHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(pkgAuth.HeaderOrgID, "hanzo")
	req.Header.Set(pkgAuth.HeaderUserID, "z")
	req.Header.Set(pkgAuth.HeaderUserEmail, "z@hanzo.ai")
	req.Header.Set(iammiddleware.HeaderUserIsAdmin, "true")
	req.Header.Set(iammiddleware.HeaderRoles, "admin, owner")
	c := &gin.Context{Request: req}

	got := iammiddleware.GetIAMClaims(c)
	if got == nil {
		t.Fatal("GetIAMClaims must be non-nil with headers")
	}
	if got.Owner != "hanzo" {
		t.Errorf("Owner = %q, want hanzo", got.Owner)
	}
	if got.Subject != "z" {
		t.Errorf("Subject = %q, want z", got.Subject)
	}
	if got.Email != "z@hanzo.ai" {
		t.Errorf("Email = %q, want z@hanzo.ai", got.Email)
	}
	if !got.IsAdmin {
		t.Error("IsAdmin = false, want true")
	}
	if len(got.Roles) != 2 || got.Roles[0] != "admin" || got.Roles[1] != "owner" {
		t.Errorf("Roles = %v, want [admin owner]", got.Roles)
	}
}

func TestGetIAMClaimsFailsClosedOnMissingIsAdmin(t *testing.T) {
	// Missing X-User-IsAdmin -> IsAdmin=false. Spoofed gibberish does
	// NOT escalate.
	gin.SetMode(gin.TestMode)
	for _, val := range []string{"", "yes", "1", "TRUE\nX-User-IsAdmin: true"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set(pkgAuth.HeaderUserID, "z")
		if val != "" {
			req.Header.Set(iammiddleware.HeaderUserIsAdmin, val)
		}
		c := &gin.Context{Request: req}
		got := iammiddleware.GetIAMClaims(c)
		if got.IsAdmin {
			t.Errorf("IsAdmin = true for X-User-IsAdmin=%q, want false (fail-closed)", val)
		}
	}
}

func TestGetIAMClaimsTestInjectionWins(t *testing.T) {
	// Tests can pre-populate "iam_claims" on the gin context to inject
	// arbitrary claim shapes. Headers are ignored when iam_claims is set.
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(iammiddleware.HeaderUserIsAdmin, "true")
	c := &gin.Context{Request: req}
	c.Set("iam_claims", &auth.IAMClaims{Owner: "injected"})

	got := iammiddleware.GetIAMClaims(c)
	if got.Owner != "injected" {
		t.Errorf("Owner = %q, want injected (test injection must win)", got.Owner)
	}
	if got.IsAdmin {
		t.Error("IsAdmin = true; injection ignored, want false (header path bypassed)")
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
