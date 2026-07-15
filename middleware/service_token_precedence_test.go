// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/middleware/svcorg"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// seedInMemDatastore installs a fresh in-memory default datastore so the
// service-token branch's svcorg.Resolve (GetOrCreate on the org table) succeeds
// and the request reaches the handler — the funded path — rather than 503ing on a
// resolve failure. Mirrors svcorg/resolver_test.go's setup.
func seedInMemDatastore(t *testing.T) {
	t.Helper()
	mgr, err := db.NewManager(&db.Config{DataDir: t.TempDir(), EnableVectorSearch: false, IsDev: true})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	shared, err := mgr.Org("system") // Organization is a global kind → shared default DB.
	if err != nil {
		t.Fatalf("mgr.Org(system): %v", err)
	}
	datastore.SetDefaultDB(shared)
	t.Cleanup(func() { datastore.SetDefaultDB(nil) })
}

// stampIAMForS2S mimics IAMTokenRequired's effect for an IN-PROCESS S2S dispatch:
// the metering/billing request carries X-Org-Id (to select the tenant namespace),
// so IAMTokenRequired stamps iam_authenticated=true; it carries NO X-User-Permissions
// (an S2S call has no per-user scope), so the minted permissions are ZERO. This is
// the exact context state under which the bug fired.
func stampIAMForS2S(c *gin.Context) {
	c.Set("iam_authenticated", true)
	c.Set("permissions", bit.Field(0))
	c.Next()
}

// TestTokenRequired_ServiceTokenBeatsIAMScopeGate is the core regression for the
// completions-500 incident: an in-process S2S metering/billing dispatch carries the
// verified COMMERCE_SERVICE_TOKEN *and* an X-Org-Id header. The X-Org-Id makes
// IAMTokenRequired stamp iam_authenticated=true with ZERO permissions, so on an
// Admin-masked billing route (/v1/billing/{balance,tier,usage}: api.Use(TokenRequired(
// permission.Admin))) the IAM scope gate used to 403 "IAM principal lacks required
// permission scope" — the service token was never consulted — and the completions
// edge rendered that internal failure as a client 500. The service token must WIN:
// possession of the KMS-sourced secret is proof of trust, so the request authenticates
// as the service, not a scope-less per-user IAM principal.
func TestTokenRequired_ServiceTokenBeatsIAMScopeGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const svc = "svc-token-precedence-abc123"
	t.Setenv("COMMERCE_SERVICE_TOKEN", svc)
	seedInMemDatastore(t)
	svcorg.Invalidate("acme")

	var sawServiceToken bool
	reached := false
	eng := gin.New()
	eng.Use(stampIAMForS2S)
	eng.GET("/v1/billing/balance", TokenRequired(permission.Admin), func(c *gin.Context) {
		sawServiceToken = IsServiceToken(c)
		reached = true
		c.JSON(http.StatusOK, gin.H{"available": 6875})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance?user=acme", nil)
	req.Header.Set("Authorization", "Bearer "+svc)
	req.Header.Set("X-Org-Id", "acme")
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !reached {
		t.Fatalf("service-token S2S dispatch on an Admin-masked route must reach the handler: "+
			"status=%d reached=%v, want 200,true (a 403 means the IAM scope gate wrongly beat the service token)",
			w.Code, reached)
	}
	if !sawServiceToken {
		t.Fatal("IsServiceToken(c)=false: the S2S dispatch must authenticate as the trusted service, not a per-user IAM principal")
	}
}

// TestTokenRequired_UserIAMScopeGateUnchanged proves the fix does NOT loosen the gate
// for real external callers: a genuine low-scope IAM principal (perms=0) with NO
// service token still fails the Admin mask (403), and a properly-scoped one passes.
// Only possession of the service token — which no external caller holds — takes the
// service path.
func TestTokenRequired_UserIAMScopeGateUnchanged(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "some-real-service-token")

	// Low-scope IAM user, no service token → still 403 on the Admin mask.
	if status, reached := runGate(t, []bit.Mask{permission.Admin}, iamAuth(bit.Field(0))); status != http.StatusForbidden || reached {
		t.Fatalf("low-scope IAM user must still be 403'd on an Admin mask: status=%d reached=%v", status, reached)
	}
	// Properly-scoped IAM user, no service token → passes, exactly as before.
	if status, reached := runGate(t, []bit.Mask{permission.Admin}, iamAuth(bit.Field(permission.Admin|permission.Live))); status != http.StatusOK || !reached {
		t.Fatalf("Admin-scoped IAM user must pass: status=%d reached=%v", status, reached)
	}
}
