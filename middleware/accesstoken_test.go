// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// runGate drives TokenRequired(masks...) through a real gin engine with the
// given pre-set context state (iam_authenticated / permissions), returning the
// HTTP status and whether the protected handler was reached. COMMERCE_SERVICE_TOKEN
// is cleared so only the IAM branch under test is exercised.
func runGate(t *testing.T, masks []bit.Mask, seed func(*gin.Context)) (int, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reached := false
	eng := gin.New()
	eng.Use(func(c *gin.Context) { seed(c); c.Next() })
	eng.POST("/x", TokenRequired(masks...), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w.Code, reached
}

func iamAuth(perms bit.Field) func(*gin.Context) {
	return func(c *gin.Context) {
		c.Set("iam_authenticated", true)
		c.Set("permissions", perms)
	}
}

// TestTokenRequired_IAMBranchEnforcesMasks is the core fix-2 proof: the IAM
// branch used to bare-c.Next() for ANY authenticated principal, which made every
// masked gate (TokenRequired(Admin, Published) on the checkout money routes) a
// NO-OP — a low-privilege or forged (perms=0) IAM caller reached the money
// handlers. It now enforces the requested masks against the gateway-minted
// permissions (bit.Field.Has = intersection, so Admin|Published is satisfied by
// EITHER bit). A no-mask gate (billing) still admits any authenticated principal.
func TestTokenRequired_IAMBranchEnforcesMasks(t *testing.T) {
	published := []bit.Mask{permission.Admin, permission.Published}

	cases := []struct {
		name       string
		masks      []bit.Mask
		perms      bit.Field
		wantStatus int
		wantReach  bool
	}{
		{"admin mints (Admin|Live intersects Admin)", published,
			bit.Field(permission.Admin | permission.Live), http.StatusOK, true},
		{"published storefront principal mints", published,
			bit.Field(permission.Published | permission.Live), http.StatusOK, true},
		{"forged/low-priv perms=0 denied", published,
			bit.Field(0), http.StatusForbidden, false},
		{"read-only scope denied a mint", published,
			bit.Field(permission.ReadStore), http.StatusForbidden, false},
		{"no-mask gate admits any authed principal (billing)", nil,
			bit.Field(0), http.StatusOK, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, reached := runGate(t, tc.masks, iamAuth(tc.perms))
			if status != tc.wantStatus || reached != tc.wantReach {
				t.Fatalf("status=%d reached=%v, want status=%d reached=%v",
					status, reached, tc.wantStatus, tc.wantReach)
			}
		})
	}
}

// TestTokenRequired_BareOrgHeaderIsNotAuthenticated proves the IsIAMAuthenticated
// hardening end-to-end: a request carrying ONLY a client X-Org-Id header (no
// validated iam_authenticated, no valid token, no service token) is NOT treated
// as an IAM principal — it falls through to token auth and 401s. Before the fix,
// the bare header alone satisfied IsIAMAuthenticated and reached the handler.
func TestTokenRequired_BareOrgHeaderIsNotAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reached := false
	eng := gin.New()
	eng.POST("/x", TokenRequired(permission.Admin, permission.Published), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Org-Id", "hanzo") // spoofed org selector, no token behind it
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized || reached {
		t.Fatalf("bare X-Org-Id must not authenticate: status=%d reached=%v, want 401,false", w.Code, reached)
	}
}
