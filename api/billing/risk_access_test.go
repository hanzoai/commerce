// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_access_test.go is the regression for the gate that read like an admin
// gate and was not one.
//
// The face was mounted behind TokenRequired(permission.Admin). On the IAM path
// that bit is minted ONLY for a Hanzo platform SuperAdmin (edgeauth.permsHeader
// gives an org owner Live and deliberately NOT Admin, so no customer can mint
// platform money), so the mask did not scope the face to a merchant's own
// admins — it scoped it to Hanzo staff, and every customer this was built for
// got 403 on all eleven ops.
//
// These tests drive the REAL registration with the identity a real caller
// carries, so what they prove is the production chain.

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// caller is one identity reaching the face: the org it resolved to, the
// permission bits the edge minted for it, and the claims it carries.
type caller struct {
	org    string
	perms  bit.Mask
	claims *auth.IAMClaims
	// orgAdminHeader mints the authority header the in-process host (cloud
	// SanitizeIdentity) sets for a merchant admin, which is NOT the same header
	// the gateway sets.
	orgAdminHeader bool
}

// asCaller mounts the real face and returns a request function for one identity.
func asCaller(t *testing.T, ctx context.Context, c caller) func(method, path, body string) *http.Response {
	t.Helper()
	org := &organization.Organization{}
	org.Name = c.org
	org.Live = true

	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-access-test"})
	app.Use(func(x *zip.Ctx) error {
		x.SetContext(ctx)
		x.Locals("iam_authenticated", true)
		x.Locals("permissions", bit.Field(c.perms))
		x.Locals("organization", org)
		if c.claims != nil {
			x.Locals("iam_claims", c.claims)
		}
		return x.Continue()
	})
	RiskRoute(app)

	return func(method, path, body string) *http.Response {
		var rdr io.Reader
		if body != "" {
			rdr = bytes.NewBufferString(body)
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
		if c.orgAdminHeader {
			req.Header.Set(iammiddleware.HeaderUserIsOrgAdmin, "true")
			req.Header.Set(iammiddleware.HeaderUserOwner, c.org)
		}
		res, err := app.Fiber().Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return res
	}
}

// TestAccess_TheCustomersThisWasBuiltForCanReachIt — an ORG ADMIN, holding
// exactly what a real merchant owner holds (Live, no platform Admin bit),
// reaches every op including the ones that restrain money.
func TestAccess_TheCustomersThisWasBuiltForCanReachIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := asCaller(t, ctx, caller{
		org:    "acme",
		perms:  permission.Live, // exactly what permsHeader mints for claims.IsAdmin
		claims: &auth.IAMClaims{Owner: "acme", HomeOrg: "acme", IsAdmin: true},
	})

	if res := call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"c1","amount":4200,"currency":"usd"}`); res.StatusCode != http.StatusOK {
		t.Fatalf("an org admin got %d screening its own move", res.StatusCode)
	}
	if res := call(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"hold","subjectKind":"merchant","subject":"m1"}`); res.StatusCode != http.StatusCreated {
		t.Fatalf("an org admin got %d placing a control in its own org", res.StatusCode)
	}
	for _, path := range []string{
		"/v1/billing/risk/screens",
		"/v1/billing/risk/controls",
		"/v1/billing/risk/merchants/m1",
		"/v1/billing/risk/reserves",
		"/v1/billing/risk/reserves/entries",
	} {
		if res := call(http.MethodGet, path, ""); res.StatusCode != http.StatusOK {
			t.Fatalf("an org admin got %d on %s", res.StatusCode, path)
		}
	}
}

// TestAccess_TheInProcessHostsOrgAdminHeaderIsHonored — cloud mints
// X-User-IsOrgAdmin for a merchant admin and reserves X-User-IsAdmin for a
// SuperAdmin, so a gate reading only the latter 403s every customer that
// reaches commerce through cloud. One predicate, both spellings.
func TestAccess_TheInProcessHostsOrgAdminHeaderIsHonored(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := asCaller(t, ctx, caller{
		org:            "acme",
		perms:          permission.Live,
		claims:         &auth.IAMClaims{Owner: "acme", HomeOrg: "acme"}, // IsAdmin false
		orgAdminHeader: true,
	})
	if res := call(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"block","subjectKind":"merchant","subject":"m1"}`); res.StatusCode != http.StatusCreated {
		t.Fatalf("the in-process host's org admin got %d placing a control", res.StatusCode)
	}
}

// TestAccess_AnOrgMemberScreensButDoesNotRestrain — the rule in one sentence:
// every op needs the org principal, the ops that restrain money need the org's
// own admin.
func TestAccess_AnOrgMemberScreensButDoesNotRestrain(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := asCaller(t, ctx, caller{
		org:    "acme",
		perms:  permission.Live,
		claims: &auth.IAMClaims{Owner: "acme", HomeOrg: "acme"}, // an ordinary member
	})

	if res := call(http.MethodPost, "/v1/billing/risk/screen",
		`{"stage":"payment","subjectKind":"customer","subject":"c1","amount":100,"currency":"usd"}`); res.StatusCode != http.StatusOK {
		t.Fatalf("an org member got %d screening: reads and screening are the integration surface", res.StatusCode)
	}
	if res := call(http.MethodGet, "/v1/billing/risk/screens", ""); res.StatusCode != http.StatusOK {
		t.Fatalf("an org member got %d listing its org's screens", res.StatusCode)
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/billing/risk/controls", `{"effect":"block","subjectKind":"merchant","subject":"m1"}`},
		{http.MethodDelete, "/v1/billing/risk/controls/xyz", ""},
		{http.MethodPost, "/v1/billing/risk/merchants/m1/review", `{"act":true}`},
	} {
		if res := call(c.method, c.path, c.body); res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s: an ordinary member got %d, want 403 — restraining money is an admin's act",
				c.method, c.path, res.StatusCode)
		}
	}
}

// TestAccess_APlatformSuperAdminStillReachesIt — SuperAdmin is the cross-tenant
// EXCEPTION, not the only door. The door it had must not close behind the fix.
func TestAccess_APlatformSuperAdminStillReachesIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	call := asCaller(t, ctx, caller{
		org:    "acme",
		perms:  permission.Admin | permission.Live,
		claims: &auth.IAMClaims{Owner: "acme", HomeOrg: "admin"},
	})
	if res := call(http.MethodPost, "/v1/billing/risk/controls",
		`{"effect":"block","subjectKind":"merchant","subject":"m1"}`); res.StatusCode != http.StatusCreated {
		t.Fatalf("a platform SuperAdmin got %d", res.StatusCode)
	}
}

// TestAccess_NoTenantIsRefused — off the HTTP path there is no principal, and a
// missing fact must never read as authority.
func TestAccess_NoTenantIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-notenant-test"})
	app.Use(func(x *zip.Ctx) error {
		x.SetContext(ctx)
		x.Locals("iam_authenticated", true)
		x.Locals("permissions", bit.Field(permission.Live))
		return x.Continue()
	})
	RiskRoute(app)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/risk/screens", nil)
	res, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("a request with no tenant got %d, want 403", res.StatusCode)
	}
}

// TestAccess_AnUnauthenticatedCallerIsRefused — the gate is no mask, not no
// gate.
func TestAccess_AnUnauthenticatedCallerIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	app := zip.New(zip.Config{DisableStartupMessage: true, AppName: "risk-anon-test"})
	app.Use(func(x *zip.Ctx) error {
		x.SetContext(ctx)
		return x.Continue()
	})
	RiskRoute(app)

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/billing/risk/screens", ""},
		{http.MethodPost, "/v1/billing/risk/screen", `{"stage":"payment","subjectKind":"customer","subject":"c1"}`},
		{http.MethodPost, "/v1/billing/risk/controls", `{"effect":"block","subjectKind":"merchant","subject":"m1"}`},
	} {
		var rdr io.Reader
		if c.body != "" {
			rdr = bytes.NewBufferString(c.body)
		}
		req := httptest.NewRequest(c.method, c.path, rdr)
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Fiber().Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", c.method, c.path, err)
		}
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s: an unauthenticated caller got %d, want 401", c.method, c.path, res.StatusCode)
		}
	}
}
