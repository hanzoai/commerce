// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// TestEdgeAuth_StripsSpoofedIdentity proves the core security fix: at a
// directly-exposed edge a client-supplied X-Org-Id (and friends) must be
// removed so it can't impersonate an org via the header-trust path.
func TestEdgeAuth_StripsSpoofedIdentity(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	h := EdgeAuth()

	req := httptest.NewRequest("GET", "/v1/billing/balance?user=hanzo&currency=usd", nil)
	req.Header.Set("X-Org-Id", "hanzo")
	req.Header.Set("X-User-Permissions", "16") // spoofed admin bit
	req.Header.Set("X-User-IsAdmin", "true")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	h(c)

	for _, k := range []string{"X-Org-Id", "X-User-Permissions", "X-User-IsAdmin"} {
		if got := c.Request.Header.Get(k); got != "" {
			t.Fatalf("EdgeAuth(enabled) must strip spoofed %s, got %q", k, got)
		}
	}
}

// TestEdgeAuth_DisabledIsNoOp proves gateway-fronted deployments are
// unaffected: with the flag off, identity headers pass through untouched.
func TestEdgeAuth_DisabledIsNoOp(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "")
	h := EdgeAuth()

	req := httptest.NewRequest("GET", "/v1/billing/balance", nil)
	req.Header.Set("X-Org-Id", "hanzo")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	h(c)

	if got := c.Request.Header.Get("X-Org-Id"); got != "hanzo" {
		t.Fatalf("EdgeAuth(disabled) must be a no-op, X-Org-Id=%q", got)
	}
}

// TestEdgeAuth_NilClientNeverMints proves fail-closed: a JWT-shaped token
// with no verifier (nil client) yields NO minted identity — the spoofed
// header is stripped and nothing is trusted in its place.
func TestEdgeAuth_NilClientNeverMints(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	h := EdgeAuth()

	req := httptest.NewRequest("GET", "/v1/billing/balance", nil)
	req.Header.Set("Authorization", "Bearer a.b.c")
	req.Header.Set("X-Org-Id", "evil")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	h(c)

	if got := c.Request.Header.Get("X-Org-Id"); got != "" {
		t.Fatalf("nil verifier must not mint identity, X-Org-Id=%q", got)
	}
}

// TestLockBillingSubject proves per-org isolation: the billing subject is
// pinned to the caller's own org slug, other query params are preserved.
func TestLockBillingSubject(t *testing.T) {
	req := httptest.NewRequest("GET",
		"/v1/billing/transactions?user=victim/other&userId=x&customerId=y&limit=5", nil)
	lockBillingSubject(req, "hanzo")

	q := req.URL.Query()
	for _, k := range []string{"user", "userId", "customerId"} {
		if q.Get(k) != "hanzo" {
			t.Fatalf("%s must be locked to org slug, got %q", k, q.Get(k))
		}
	}
	if q.Get("limit") != "5" {
		t.Fatalf("unrelated query param clobbered, limit=%q", q.Get("limit"))
	}
}

func TestLooksLikeJWTAndBearer(t *testing.T) {
	cases := map[string]bool{"a.b.c": true, "hk-abc123": false, "": false, "a.b": false}
	for tok, want := range cases {
		if got := looksLikeJWT(tok); got != want {
			t.Fatalf("looksLikeJWT(%q)=%v want %v", tok, got, want)
		}
	}
	if got := bearerToken("Bearer xyz"); got != "xyz" {
		t.Fatalf("bearerToken Bearer parse = %q", got)
	}
	if got := bearerToken("xyz"); got != "" {
		t.Fatalf("bearerToken non-bearer must be empty, got %q", got)
	}
}
