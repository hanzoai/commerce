// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// runMintGate drives the EXACT production money-mint middleware chain —
// TokenRequired(permission.Admin) THEN PlatformOnly() — that api/billing.Route
// mounts on /credit, /deposit, /refund, /credits and
// /customer-balance/adjustments, to a sentinel handler. Returns the HTTP status and whether the
// sentinel was reached. seed pre-sets context state exactly as the real upstream
// middleware would (EdgeAuth/IAMTokenRequired mint iam_authenticated + permissions
// + iam_claims); the service-token branch is exercised via a real Bearer + env.
func runMintGate(t *testing.T, seed func(*zip.Ctx), reqSetup func(*http.Request)) (int, bool) {
	t.Helper()

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if seed != nil {
		app.Use(zip.H(func(c *zip.Ctx) error { seed(c); return c.Next() }))
	}
	app.Use(TokenRequired(permission.Admin))
	app.Use(PlatformOnly())
	app.Post("/x", func(c *zip.Ctx) error { reached = true; return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if reqSetup != nil {
		reqSetup(req)
	}
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp.StatusCode, reached
}

func iamIdentity(perms bit.Field, claims *auth.IAMClaims) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", perms)
		c.Locals("iam_claims", claims)
	}
}

// TestPlatformOnly_OrgAdminDeniedMint is THE C1 proof at the gate: an ORG-level
// admin (org owner — org-level IsAdmin=true, so the gateway mints Admin|Live, but
// IsSuperAdmin()=false) PASSES TokenRequired(Admin) yet is DENIED (403) by
// PlatformOnly on the money-mint chain; the handler is never reached. This is the
// exact principal (e.g. maxpower) that could previously self-mint unlimited balance.
func TestPlatformOnly_OrgAdminDeniedMint(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	orgAdmin := &auth.IAMClaims{Owner: "maxpower", IsAdmin: true} // org owner, NOT global
	status, reached := runMintGate(t, iamIdentity(bit.Field(permission.Admin|permission.Live), orgAdmin), nil)
	if status != http.StatusForbidden || reached {
		t.Fatalf("org-admin on money-mint chain: status=%d reached=%v, want 403 & not-reached (this was the C1 hole)", status, reached)
	}
}

// TestPlatformOnly_SuperAdminMints proves a PLATFORM (global) admin — the explicit
// the reserved "admin" org (owner=="admin") — passes the same chain
// (the legitimate human-superadmin path RED's spec requires to still work).
func TestPlatformOnly_SuperAdminMints(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	for _, gc := range []*auth.IAMClaims{
		{HomeOrg: "admin", Owner: "hanzo"}, // home org grants sudo even from a non-admin effective org
		{Owner: "admin"},                   // the global-admin org
	} {
		status, reached := runMintGate(t, iamIdentity(bit.Field(permission.Admin|permission.Live), gc), nil)
		if status != http.StatusOK || !reached {
			t.Fatalf("global admin %+v: status=%d reached=%v, want 200 & reached", gc, status, reached)
		}
	}
}

// TestPlatformOnly_ServiceTokenMints proves the internal service token (cloud-api →
// commerce, COMMERCE_SERVICE_TOKEN) passes the chain — the legitimate money path
// that MUST keep working. Drives the REAL service-token branch of TokenRequired
// (which stamps the marker PlatformOnly reads).
func TestPlatformOnly_ServiceTokenMints(t *testing.T) {
	const tok = "svc-secret-xyz"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	// The service-token branch resolves the caller-named org via c.Context().
	seed := func(c *zip.Ctx) { c.SetContext(ctx) }
	status, reached := runMintGate(t, seed, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+tok)
		r.Header.Set("X-Org-Id", "svc-org")
	})
	if status != http.StatusOK || !reached {
		t.Fatalf("service token: status=%d reached=%v, want 200 & reached (money path must keep working)", status, reached)
	}
}

// TestPlatformOnly_AdminBitAloneDenied pins the gate's core invariant in isolation:
// holding the org-level Admin bit — or being merely IAM-authenticated as an org
// owner — is NOT sufficient; only the service-token marker or SuperAdmin (owner=="admin") passes.
// This is the anti-conflation that closes C1: a legacy per-org access token and an
// org owner, both of which carry Admin, are refused. Fail-closed on empty context.
func TestPlatformOnly_AdminBitAloneDenied(t *testing.T) {
	cases := []struct {
		name string
		seed func(*zip.Ctx)
	}{
		{"nothing set (fail-closed)", func(c *zip.Ctx) {}},
		{"org-level Admin permission bit only (legacy access token)", func(c *zip.Ctx) {
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		}},
		{"iam org-admin, not global", func(c *zip.Ctx) {
			c.Locals("iam_authenticated", true)
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
			c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			app := zip.New(zip.Config{DisableStartupMessage: true})
			app.Use(zip.H(func(c *zip.Ctx) error { tc.seed(c); return c.Next() }))
			app.Use(PlatformOnly())
			app.Post("/x", func(c *zip.Ctx) error {
				reached = true
				return c.NoContent(http.StatusOK)
			})
			resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodPost, "/x", nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != http.StatusForbidden || reached {
				t.Fatalf("%s: status=%d reached=%v, want 403 & not-reached", tc.name, resp.StatusCode, reached)
			}
		})
	}
}

// TestIsServiceToken_FailClosed proves the marker predicate never reads a
// non-service request as a service caller: absent or non-bool value → false.
func TestIsServiceToken_FailClosed(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodPost, "/x")
	if IsServiceToken(c) {
		t.Fatal("empty context: IsServiceToken must be false (fail-closed)")
	}
	c.Locals(ctxKeyServiceToken, "true") // wrong type, not bool
	if IsServiceToken(c) {
		t.Fatal("non-bool marker: IsServiceToken must be false (fail-closed)")
	}
	c.Locals(ctxKeyServiceToken, true)
	if !IsServiceToken(c) {
		t.Fatal("bool true marker: IsServiceToken must be true")
	}
}
