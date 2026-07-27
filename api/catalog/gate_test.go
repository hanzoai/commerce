package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// probeGate drives the REAL chain AdminRoute mounts — TokenRequired(permission.Admin)
// — to a sentinel that reports whether the named gate admitted the caller. The
// service-token branch is exercised the way production reaches it, a real Bearer
// against a real env, never by stamping the marker by hand: the marker is
// middleware's own private state and a test that writes it directly would keep
// passing after the real branch stopped setting it.
func probeGate(t *testing.T, gate func(*zip.Ctx) bool, seed func(*zip.Ctx), bearer string) (int, bool) {
	t.Helper()

	admitted := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if seed != nil {
		app.Use(func(c *zip.Ctx) error { seed(c); return c.Next() })
	}
	app.Use(middleware.TokenRequired(permission.Admin))
	app.Post("/x", func(c *zip.Ctx) error {
		if !gate(c) {
			return nil
		}
		admitted = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp.StatusCode, admitted
}

func iamIdentity(claims *auth.IAMClaims) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", claims)
	}
}

// TestSyncGate_AdmitsTheServiceToken is the regression. The sync doors were
// gated on SuperAdmin CLAIMS, which the internal service token does not carry,
// so the scheduled run this catalog depends on could only ever 403 — and the
// model rows were in fact empty in production. A sync a human has to start is a
// catalog that goes stale.
func TestSyncGate_AdmitsTheServiceToken(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Setenv("COMMERCE_SERVICE_TOKEN", "s3cret-service-token")
	status, admitted := probeGate(t, requirePlatform, nil, "s3cret-service-token")
	if status != http.StatusOK || !admitted {
		t.Fatalf("service token on the sync gate: status=%d admitted=%v, want 200 & admitted", status, admitted)
	}
}

// TestSyncGate_RefusesAnOrgAdmin holds the boundary the widening must not cross:
// an org owner carries org-level IsAdmin and clears TokenRequired(Admin), but the
// platform catalog is cross-tenant data and is not theirs to sync.
func TestSyncGate_RefusesAnOrgAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	orgAdmin := &auth.IAMClaims{Owner: "acme", IsAdmin: true}
	status, admitted := probeGate(t, requirePlatform, iamIdentity(orgAdmin), "")
	if status != http.StatusForbidden || admitted {
		t.Fatalf("org admin on the sync gate: status=%d admitted=%v, want 403 & refused", status, admitted)
	}
}

// TestEditGate_RefusesTheServiceToken is the other half of the widening, and the
// one that keeps it honest: the token may land upstream COST on the sync door and
// nothing more. Editing an entry — which is how a retail price is set — stays a
// named human in the admin org, so possession of a service credential can never
// change what a customer is charged.
func TestEditGate_RefusesTheServiceToken(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Setenv("COMMERCE_SERVICE_TOKEN", "s3cret-service-token")
	status, admitted := probeGate(t, requireSuperAdmin, nil, "s3cret-service-token")
	if status != http.StatusForbidden || admitted {
		t.Fatalf("service token on the entry-edit gate: status=%d admitted=%v, want 403 & refused", status, admitted)
	}
}

// TestSyncGate_AdmitsASuperAdmin: a platform admin keeps the door they had, so
// the change is purely additive to who may run a sync.
func TestSyncGate_AdmitsASuperAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	superAdmin := &auth.IAMClaims{Owner: "admin", IsAdmin: true}
	status, admitted := probeGate(t, requirePlatform, iamIdentity(superAdmin), "")
	if status != http.StatusOK || !admitted {
		t.Fatalf("super admin on the sync gate: status=%d admitted=%v, want 200 & admitted", status, admitted)
	}
}
