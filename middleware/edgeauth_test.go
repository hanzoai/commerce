// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
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

// TestEdgeAuth_StripsGlobalAdminSpoof proves the PLATFORM-superadmin header
// is stripped at the edge. After the global-admin/org-admin split,
// GetIAMClaims reads X-User-IsGlobalAdmin straight off the request and
// GlobalAdmin() trusts it — so if EdgeAuth did NOT strip it, an in-cluster
// caller could forge `X-User-IsGlobalAdmin: true` and pass every cross-org
// gate (the exact /_/commerce/tenants escalation). It MUST be in the strip set.
func TestEdgeAuth_StripsGlobalAdminSpoof(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	h := EdgeAuth()

	req := httptest.NewRequest("POST", "/_/commerce/tenants", nil)
	req.Header.Set("X-Org-Id", "admin")
	req.Header.Set("X-User-IsGlobalAdmin", "true") // forged platform superadmin
	req.Header.Set("X-User-IsAdmin", "true")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	h(c)

	for _, k := range []string{"X-Org-Id", "X-User-IsGlobalAdmin", "X-User-IsAdmin"} {
		if got := c.Request.Header.Get(k); got != "" {
			t.Fatalf("EdgeAuth must strip spoofed %s, got %q", k, got)
		}
	}
}

// TestEdgeAuth_PreservesServiceTokenOrg proves the money path is untouched:
// cloud-api -> commerce per-org billing authenticates with an OPAQUE Bearer
// service token (not a JWT) and names the org via X-Hanzo-Org. EdgeAuth must
// (a) strip any forged X-Org-Id, (b) NOT strip X-Hanzo-Org (it is not an
// identity header), and (c) never abort — the opaque token is not a JWT, so
// minting is skipped and the request flows through to the service-token
// authorizer. Breaking any of these breaks per-org billing.
func TestEdgeAuth_PreservesServiceTokenOrg(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	h := EdgeAuth()

	req := httptest.NewRequest("POST", "/v1/billing/usage", nil)
	req.Header.Set("Authorization", "Bearer st_opaque_service_token_not_a_jwt")
	req.Header.Set("X-Hanzo-Org", "maxpower") // trusted service-token org selector
	req.Header.Set("X-Org-Id", "admin")       // forged identity — must be stripped

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	h(c)

	if c.IsAborted() {
		t.Fatal("EdgeAuth must NOT abort an opaque service-token request (money path)")
	}
	if got := c.Request.Header.Get("X-Org-Id"); got != "" {
		t.Fatalf("forged X-Org-Id must be stripped, got %q", got)
	}
	if got := c.Request.Header.Get("X-Hanzo-Org"); got != "maxpower" {
		t.Fatalf("X-Hanzo-Org must be preserved for per-org billing, got %q", got)
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

// TestIsGlobalAdmin proves the global-admin gate: only the explicit
// isGlobalAdmin claim or membership in the "admin" org qualifies. A plain
// org-level IsAdmin (e.g. an org owner) must NOT — trusting it would let any
// org owner read another org via ?org=.
func TestIsGlobalAdmin(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.IAMClaims
		want   bool
	}{
		{"nil", nil, false},
		{"admin-org", &auth.IAMClaims{Owner: "admin"}, true},
		{"admin-org-mixedcase", &auth.IAMClaims{Owner: "Admin"}, true},
		{"global-flag", &auth.IAMClaims{Owner: "hanzo", IsGlobalAdmin: true}, true},
		{"org-admin-not-global", &auth.IAMClaims{Owner: "maxpower", IsAdmin: true}, false},
		{"plain-user", &auth.IAMClaims{Owner: "hanzo"}, false},
	}
	for _, tc := range cases {
		if got := isGlobalAdmin(tc.claims); got != tc.want {
			t.Fatalf("%s: isGlobalAdmin=%v want %v", tc.name, got, tc.want)
		}
	}
}

// TestResolveBillingSubject_AdminOverride: a global admin may retarget the
// billing view to another org via ?org=, and the namespace follows it.
func TestResolveBillingSubject_AdminOverride(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/billing/balance?user=admin/z&org=hanzo&currency=usd", nil)
	subject, override := resolveBillingSubject(req, &auth.IAMClaims{Owner: "admin", IsGlobalAdmin: true})
	if subject != "hanzo" || !override {
		t.Fatalf("admin ?org=hanzo => subject=%q override=%v, want hanzo,true", subject, override)
	}
	if req.URL.Query().Has("org") {
		t.Fatalf("?org must be stripped from the query, got %q", req.URL.RawQuery)
	}
	if req.URL.Query().Get("currency") != "usd" {
		t.Fatalf("unrelated query param clobbered: %q", req.URL.RawQuery)
	}
}

// TestResolveBillingSubject_NonAdminIgnoresOverride is the isolation proof:
// a non-global-admin cannot use ?org= to read another org. The override is
// stripped and the subject stays pinned to the caller's own org.
func TestResolveBillingSubject_NonAdminIgnoresOverride(t *testing.T) {
	// Dave: org owner with IsAdmin=true (org-level), NOT a global admin.
	req := httptest.NewRequest("GET", "/v1/billing/balance?org=hanzo&currency=usd", nil)
	subject, override := resolveBillingSubject(req, &auth.IAMClaims{Owner: "maxpower", IsAdmin: true})
	if subject != "maxpower" || override {
		t.Fatalf("non-admin ?org=hanzo => subject=%q override=%v, want maxpower,false (isolation)", subject, override)
	}
	if req.URL.Query().Has("org") {
		t.Fatalf("?org must be stripped even for non-admins, got %q", req.URL.RawQuery)
	}
}

// TestResolveBillingSubject_NoOverride: without ?org, every caller (incl. a
// global admin) defaults to their own org and the namespace is untouched.
func TestResolveBillingSubject_NoOverride(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/billing/balance?currency=usd", nil)
	subject, override := resolveBillingSubject(req, &auth.IAMClaims{Owner: "admin", IsGlobalAdmin: true})
	if subject != "admin" || override {
		t.Fatalf("no ?org => subject=%q override=%v, want admin,false", subject, override)
	}
}

// TestResolveBillingSubject_RejectsBadSlug: a malformed ?org is discarded
// (treated as absent), so even an admin stays on their own org rather than a
// smuggled value.
func TestResolveBillingSubject_RejectsBadSlug(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/billing/balance?org=hanzo%2Fevil", nil)
	subject, override := resolveBillingSubject(req, &auth.IAMClaims{Owner: "admin", IsGlobalAdmin: true})
	if subject != "admin" || override {
		t.Fatalf("bad ?org slug => subject=%q override=%v, want admin,false", subject, override)
	}
}

func TestValidOrgSlug(t *testing.T) {
	good := []string{"hanzo", "admin", "max-power", "a", "org123"}
	bad := []string{"", "-lead", "Caps", "has space", "slash/x", "under_score", "dot.org"}
	for _, s := range good {
		if !validOrgSlug(s) {
			t.Fatalf("validOrgSlug(%q)=false, want true", s)
		}
	}
	for _, s := range bad {
		if validOrgSlug(s) {
			t.Fatalf("validOrgSlug(%q)=true, want false", s)
		}
	}
}
