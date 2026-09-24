package middleware

import (
	"net/http/httptest"
	"testing"
)

// X-User-IsOrgAdmin is an authority header: iammiddleware.ActsAsOrgAdmin and
// isOrgAdmin both read it. The edge's strip set must remove every authority header
// the downstream trusts, so none survives from the client side.
func TestRed_EdgeAuth_StripsOrgAdminHeader(t *testing.T) {
	t.Setenv("COMMERCE_EDGE_AUTH", "true")

	req := httptest.NewRequest("POST", "/v1/billing/subscribe/card", nil)
	req.Header.Set("X-User-IsOrgAdmin", "true")

	_, headers := runEdgeAuth(t, req, "X-User-IsOrgAdmin")
	if got := headers["X-User-IsOrgAdmin"]; got != "" {
		t.Fatalf("EdgeAuth(enabled) left client-sent X-User-IsOrgAdmin=%q in place", got)
	}
}
