// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// TestGroupUseIsMembershipScopedNotPrefix is the EXPERIMENT behind Mint's shape,
// kept as a test because the constraint is invisible in the Group/Use API and
// would otherwise be rediscovered the hard way.
//
// The intuitive way to express an authz class is "a group carrying its own Use":
//
//	mint := api.Group("")
//	mint.Use(PlatformOnly())      // ← gates the group's own routes
//	mint.Post("/deposit", Deposit)
//
// Through zip v1.18 that shape LEAKED. Use matched by PATH PREFIX, and a bare
// sub-group inherited its parent's prefix verbatim (getGroupPath(prefix, "") ==
// prefix), so the gate registered at "/v1/billing" — the SAME prefix as `api` —
// and ran for every neighbouring route under it, 403'ing the org-admin reads
// (GET /balance, /invoices, /subscriptions, …) that must stay reachable. Mint
// therefore prepends the gate per-route, which gates exactly the declared routes.
//
// zip v1.19 INVERTED this: a definition's middleware wraps the routes in its OWN
// subtree, so the gate no longer reaches a sibling registered on the parent. The
// gated-sub-group shape is now sound, and simpler than the decorator's per-route
// prepend — but adopting it is a change to a money gate, and it is only half of
// Mint anyway (the registry still needs a decorator to record what was declared),
// so it is deliberately NOT taken here. Mint's per-route prepend remains correct
// under either scoping: it gates precisely the routes registered through it.
//
// The test therefore pins the CURRENT semantics in both directions, because
// "did not fire" alone would also pass for a middleware that never runs at all.
func TestGroupUseIsMembershipScopedNotPrefix(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	api := app.Group("/v1").Group("billing")

	// A bare sub-group of api with its own middleware — the "authz is middleware
	// on a group" shape.
	sub := api.Group("")
	var sawPaths []string
	sub.Use(zip.H(func(c *zip.Ctx) error {
		// CLONE: c.Path() aliases fasthttp's pooled request buffer, so a retained
		// path is rewritten in place by the NEXT request — a recorded
		// "/v1/billing/deposit" silently becomes "/v1/billing/balance" and the
		// probe reports a leak that never happened.
		sawPaths = append(sawPaths, strings.Clone(c.Path()))
		return c.Next()
	}))
	sub.Post("/deposit", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "deposit") })

	// A sibling NON-mint read, registered on the PARENT group.
	api.Get("/balance", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "balance") })

	saw := func(path string) bool {
		for _, p := range sawPaths {
			if p == path {
				return true
			}
		}
		return false
	}
	get := func(method, path string) int {
		resp, err := app.Fiber().Test(httptest.NewRequest(method, path, nil))
		if err != nil {
			t.Fatalf("Test %s %s: %v", method, path, err)
		}
		return resp.StatusCode
	}

	// (1) The sub-group's OWN route is wrapped — otherwise (2) proves nothing.
	if code := get(http.MethodPost, "/v1/billing/deposit"); code != http.StatusOK {
		t.Fatalf("POST /v1/billing/deposit: status=%d, want 200", code)
	}
	if !saw("/v1/billing/deposit") {
		t.Fatalf("a sub-group's Use did NOT wrap the sub-group's own route (saw %v).\n"+
			"    Group middleware is not running at all, so the scoping this test reports is meaningless.", sawPaths)
	}

	// (2) A sibling on the PARENT is NOT wrapped: membership, not prefix.
	if code := get(http.MethodGet, "/v1/billing/balance"); code != http.StatusOK {
		t.Fatalf("GET /v1/billing/balance: status=%d, want 200", code)
	}
	if saw("/v1/billing/balance") {
		t.Fatalf("a bare sub-group's Use FIRED on the parent's sibling route (saw %v).\n"+
			"    Use has reverted to PREFIX scoping: `api.Group(\"\").Use(gate)` would gate ALL of\n"+
			"    /v1/billing, every org-admin read included. Mint's per-route prepend is then the\n"+
			"    only safe shape — do not convert it to a gated sub-group.", sawPaths)
	}
	t.Logf("CONFIRMED membership scoping: sub-group Use wrapped %v, and left the parent's sibling alone.", sawPaths)
}

// TestMintRecordsFullPathAndGates proves the two halves of the single
// declaration: the route is recorded with the full path fiber routes on, and the
// gate is really in its chain (an ungated caller is refused before the handler).
func TestMintRecordsFullPathAndGates(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	app := zip.New(zip.Config{DisableStartupMessage: true})
	api := app.Group("/v1").Group("billing")

	reached := false
	Mint(api, "/v1/billing").Post("/mint-probe", func(c *zip.Ctx) error {
		reached = true
		return c.JSON(http.StatusOK, "minted")
	})

	// Recorded with the full path fiber routes on.
	want := MintRoute{Method: http.MethodPost, Path: "/v1/billing/mint-probe"}
	found := false
	for _, r := range MintRoutes() {
		if r == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("MintRoutes() does not contain %+v; got %+v", want, MintRoutes())
	}

	// Gated: no service token and no SuperAdmin claim → 403, handler never runs.
	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodPost, "/v1/billing/mint-probe", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 — Mint must gate every route it registers", resp.StatusCode)
	}
	if reached {
		t.Fatal("handler ran despite the gate — Mint's gate is not first in the chain")
	}
}

// TestMintRegistryIsASet proves re-registering the same route is idempotent.
// Route() runs once in production but many times across the test suite, and a
// consumer comparing against the registry must not see duplicates.
func TestMintRegistryIsASet(t *testing.T) {
	register := func() {
		app := zip.New(zip.Config{DisableStartupMessage: true})
		Mint(app.Group("/v1").Group("billing"), "/v1/billing").Post("/set-probe", func(c *zip.Ctx) error { return nil })
	}
	register()
	register()

	n := 0
	for _, r := range MintRoutes() {
		if r.Path == "/v1/billing/set-probe" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("registered the same route twice → %d entries, want 1 (the registry is a set)", n)
	}
}

// TestMintUseIsRefused pins the boot-time refusal: Use on a Mint router would
// delegate to the underlying group and leak the gate onto its neighbours (see
// TestGroupUseIsPrefixScopedNotMembership), so it panics rather than silently
// widening authz.
func TestMintUseIsRefused(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Mint(...).Use did not panic — it must refuse rather than widen the gate to the parent group")
		}
	}()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	Mint(app.Group("/v1").Group("billing"), "/v1/billing").Use(zip.H(func(c *zip.Ctx) error { return nil }))
}

// A TYPED op declared on a mint router is gated exactly as an untyped one is.
// zip asks a Router where an op should land; a decorator that answered with the
// inner router's scope would register a money route with no gate at all — this
// router's whole purpose, skipped silently.
func TestMintGatesATypedOp(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	mint := Mint(app.Group("/v1/billing"), "/v1/billing")

	type depositIn struct {
		Cents int `json:"cents"`
	}
	type depositOut struct {
		OK bool `json:"ok"`
	}
	MintOp(mint, http.MethodPost, "/deposit",
		func(context.Context, *depositIn) (*depositOut, error) { return &depositOut{OK: true}, nil })

	// Registered, and recorded — one declaration, both effects, same as Route.
	var recorded bool
	for _, r := range MintRoutes() {
		if r.Method == http.MethodPost && r.Path == "/v1/billing/deposit" {
			recorded = true
		}
	}
	if !recorded {
		t.Fatalf("typed mint op is not in MintRoutes(): %+v", MintRoutes())
	}

	// Gated: no platform principal, no deposit.
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/deposit", strings.NewReader(`{"cents":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("ungated typed mint op answered %d — PlatformOnly did not run", resp.StatusCode)
	}

	// And it is a real op: one declaration, every projection.
	if len(app.Commands()) != 1 || app.Commands()[0].Path != "/v1/billing/deposit" {
		t.Fatalf("commands = %+v, want the one op", app.Commands())
	}
	if len(app.MCPTools()) != 1 {
		t.Fatalf("tools = %+v, want the one op", app.MCPTools())
	}
}
