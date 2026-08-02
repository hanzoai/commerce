// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// admin_test.go pins WHO ADMINISTERS AN ORG — one predicate, whatever shape the
// caller arrived in and whichever surface asks.
//
// The bug it guards against is a gate that reads as "an administrator" and is
// not. permission.Admin is deliberately withheld from every org admin and
// reserved for the internal service token and platform SuperAdmins, so a route
// gated on that bit alone answers 403 to exactly the merchants it was built for.

func ctx(t *testing.T, seed func(*zip.Ctx)) *zip.Ctx {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodGet, "/v1/billing/risk/controls")
	if seed != nil {
		seed(c)
	}
	return c
}

func TestIsAdmin_TheAdminBitIsOneDoorNotTheOnlyOne(t *testing.T) {
	// 1. The verified service token and the legacy per-org access token.
	if !IsAdmin(ctx(t, func(c *zip.Ctx) {
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
	})) {
		t.Fatal("the Admin bit did not admit")
	}

	// 2. A platform SuperAdmin — the cross-tenant exception.
	if !IsAdmin(ctx(t, func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", HomeOrg: "admin", Name: "u1"})
	})) {
		t.Fatal("a platform SuperAdmin was refused")
	}

	// 3. THE ONE THE FACE EXISTS FOR: a merchant administering its own org,
	// carrying no Admin bit at all.
	if !IsAdmin(ctx(t, func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(0))
		c.Locals(iammiddleware.LocalOrgAdmin, true)
	})) {
		t.Fatal("an org's own administrator was refused — the customers this is built for get 403")
	}
}

func TestIsAdmin_FailsClosed(t *testing.T) {
	for name, seed := range map[string]func(*zip.Ctx){
		"nothing at all": nil,
		"an authenticated member of the org": func(c *zip.Ctx) {
			c.Locals("iam_authenticated", true)
			c.Locals("permissions", bit.Field(permission.Live))
		},
		"an unauthenticated caller waving the org-admin marker": func(c *zip.Ctx) {
			c.Locals(iammiddleware.LocalOrgAdmin, true)
		},
		"permissions of the wrong type": func(c *zip.Ctx) {
			c.Locals("permissions", "admin")
		},
	} {
		if IsAdmin(ctx(t, seed)) {
			t.Fatalf("%s was treated as an administrator", name)
		}
	}
}

// TestBind_CarriesTheAnswerToTypedOps — a typed op holds a context, not a
// request, so the predicate is answered once on the way in and read back there.
func TestBind_CarriesTheAnswerToTypedOps(t *testing.T) {
	org := &organization.Organization{}
	org.Name = "acme"

	for name, tc := range map[string]struct {
		seed func(*zip.Ctx)
		want bool
	}{
		"an org admin": {func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.Locals("iam_authenticated", true)
			c.Locals(iammiddleware.LocalOrgAdmin, true)
		}, true},
		"a member": {func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.Locals("iam_authenticated", true)
		}, false},
	} {
		app := zip.New(zip.Config{DisableStartupMessage: true})
		var got, resolved bool
		app.Get("/probe", func(c *zip.Ctx) error {
			tc.seed(c)
			return c.Continue()
		}, Bind(), func(c *zip.Ctx) error {
			got = AdminFrom(c.Context())
			_, resolved = OrgFrom(c.Context())
			return c.JSON(http.StatusOK, map[string]string{"ok": "1"})
		})
		if _, err := app.Fiber().Test(httptestGet("/probe")); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !resolved {
			t.Fatalf("%s: the org did not reach the op", name)
		}
		if got != tc.want {
			t.Fatalf("%s: AdminFrom=%v want %v", name, got, tc.want)
		}
	}
}

// TestAdminFrom_FailsClosedOffTheRequestPath — the CLI and op-call projections
// invoke an op with no request behind them. There is no authority there, so an
// op that needs one refuses.
func TestAdminFrom_FailsClosedOffTheRequestPath(t *testing.T) {
	if AdminFrom(context.Background()) {
		t.Fatal("a context with no request behind it carried administrator authority")
	}
}

func httptestGet(path string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	return req
}

// TestIsAdmin_TheHeaderPathCarriesTheTenantBinding is the gate on delegation.
//
// The two identity edges mint the org-admin flag under two names, and each
// names an admin OF THE CALLER'S HOME ORG. IAMTokenRequired resolves both once
// against the org the request ACTS IN and records the bound answer; reading the
// raw claim again afterwards re-derives the same authority with no tenant in
// it, so a principal that is an admin at home inherits authority over a foreign
// tenant — on the gate that places and lifts restraints on real money.
//
// It drives the HEADER path deliberately: the earlier tests seed the bound
// local directly, so the clause that reads the claim was never exercised, and
// an untested clause on a money boundary is where this went wrong.
func TestIsAdmin_TheHeaderPathCarriesTheTenantBinding(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})

	var admin, bound bool
	app.Delete("/probe", func(c *zip.Ctx) error {
		// Exactly the state IAMTokenRequired leaves behind for a caller whose
		// edge-minted org-admin flag is for its HOME org "attacker" while the
		// request acts in the EFFECTIVE org "victim".
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Live)) // no permission.Admin
		c.Locals(iammiddleware.LocalOrgAdmin, false)        // the bound answer: NOT this org's admin
		bound = iammiddleware.IsOrgAdmin(c)
		admin = IsAdmin(c)
		return c.JSON(http.StatusOK, map[string]string{"ok": "1"})
	})

	req, _ := http.NewRequest(http.MethodDelete, "/probe", nil)
	req.Header.Set("X-User-IsAdmin", "true") // the gateway mints this for an ORG admin
	req.Header.Set("X-User-Owner", "attacker")
	req.Header.Set("X-Org-Id", "victim")
	if _, err := app.Fiber().Test(req); err != nil {
		t.Fatalf("drive: %v", err)
	}

	if bound {
		t.Fatal("precondition: the bound org-admin fact should be false")
	}
	if admin {
		t.Fatal("an admin of ANOTHER org administers this one — the boundary delegated its own tenant check")
	}
}

// TestIsAdmin_ASuperAdminHeaderStillAdmits — the cross-tenant exception is the
// reserved admin org and it survives the clause that was deleted beside it.
func TestIsAdmin_ASuperAdminHeaderStillAdmits(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})

	var admin bool
	app.Delete("/probe", func(c *zip.Ctx) error {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Live))
		c.Locals(iammiddleware.LocalOrgAdmin, false)
		admin = IsAdmin(c)
		return c.JSON(http.StatusOK, map[string]string{"ok": "1"})
	})

	req, _ := http.NewRequest(http.MethodDelete, "/probe", nil)
	req.Header.Set("X-User-Owner", "admin")
	req.Header.Set("X-Org-Id", "victim")
	if _, err := app.Fiber().Test(req); err != nil {
		t.Fatalf("drive: %v", err)
	}
	if !admin {
		t.Fatal("a platform SuperAdmin arriving by header was refused")
	}
}
