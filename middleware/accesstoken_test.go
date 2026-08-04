// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// runGate drives TokenRequired(masks...) through a real zip app with the given
// pre-set context state (iam_authenticated / permissions), returning the HTTP
// status and whether the protected handler was reached. COMMERCE_SERVICE_TOKEN
// is cleared so only the IAM branch under test is exercised.
func runGate(t *testing.T, masks []bit.Mask, seed func(*zip.Ctx)) (int, bool) {
	t.Helper()
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error { seed(c); return c.Next() }))
	app.Use(TokenRequired(masks...))
	app.Post("/x", func(c *zip.Ctx) error { reached = true; return c.NoContent(http.StatusOK) })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodPost, "/x", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp.StatusCode, reached
}

func iamAuth(perms bit.Field) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", perms)
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
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(TokenRequired(permission.Admin, permission.Published))
	app.Post("/x", func(c *zip.Ctx) error {
		reached = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Org-Id", "hanzo") // spoofed org selector, no token behind it
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized || reached {
		t.Fatalf("bare X-Org-Id must not authenticate: status=%d reached=%v, want 401,false", resp.StatusCode, reached)
	}
}

// TestTokenRequired_ServiceTokenResolveFailsFast is the money-critical regression
// for the 2026-07-04 wedge: when the bearer IS the verified service token but the
// backing store cannot resolve the org, the branch must fail CLOSED and RETRYABLE
// (503) — NOT fall through to the legacy per-org-token path (which Peeks the
// 64-hex service token as a JWT → "Invalid Segments" → a misleading 401 that made
// cloud-api treat a transient DB hiccup as a bad credential and 402 customers).
// A nil default datastore forces orgpkg.Resolve to error deterministically.
func TestTokenRequired_ServiceTokenResolveFailsFast(t *testing.T) {
	const svc = "svc-token-abc123"
	t.Setenv("COMMERCE_SERVICE_TOKEN", svc)

	// No default DB installed → GetOrCreate inside orgpkg.Resolve errors, so we
	// exercise the resolve-failure branch. Use a distinct org slug so this test's
	// failure is never masked by another test's cached success.
	datastore.SetDefaultDB(nil)
	orgpkg.Invalidate("failorg")

	reached := false
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(TokenRequired()) // no-mask billing-style gate
	app.Post("/x", func(c *zip.Ctx) error {
		reached = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+svc)
	req.Header.Set("X-Org-Id", "failorg")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if reached {
		t.Fatalf("handler must NOT be reached when service-token org resolve fails")
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("verified-service-token resolve failure must be 503 (retryable), got %d; "+
			"a 401 means it fell through to the legacy Peek path (the wedge)", resp.StatusCode)
	}
}
