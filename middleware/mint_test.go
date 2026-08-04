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

// TestGroupUseIsPrefixScopedNotMembership is the EXPERIMENT that fixes Mint's
// shape, kept as a test because the constraint is invisible in the Group/Use API
// and would otherwise be rediscovered the hard way.
//
// The intuitive way to express an authz class is "a group carrying its own Use":
//
//	mint := api.Group("")
//	mint.Use(PlatformOnly())      // ← gates the group's own routes … in theory
//	mint.Post("/deposit", Deposit)
//
// That shape does NOT work on this router. fiber's Use matches by PATH PREFIX,
// not by group membership, and a BARE sub-group inherits its parent's prefix
// verbatim (getGroupPath(prefix, "") == prefix). So `mint`'s Use registers at
// "/v1/billing" — the SAME prefix as `api` — and runs for every neighbouring
// route under it. The gate would silently spread from the 16 mint routes to the
// whole billing surface, 403'ing the org-admin reads (GET /balance, /invoices,
// /subscriptions, …) that must stay reachable.
//
// zip v1.23 CLOSED that leak: Use composes over a definition's own subtree, not
// over a URL prefix, so a sibling registered against the PARENT is no longer
// wrapped. This test now holds the guarantee rather than documenting the hazard —
// it fails if prefix scoping ever returns.
//
// Mint still prepends its gate per-route and still refuses Use. That is no longer
// the ONLY thing standing between a bare `api.Group("").Use(gate)` and the whole
// billing surface, but it is the thing that does not depend on which framework
// version is pinned, and these are the routes that move money. The simpler shape
// the original test existed to rule out is now available; taking it is a separate
// change, on its own evidence.
func TestGroupUseIsMembershipScopedNotPrefix(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	api := app.Group("/v1").Group("billing")

	// A bare sub-group of api with its own middleware — the "authz is middleware
	// on a group" shape.
	sub := api.Group("")
	var sawPaths []string
	sub.Use(zip.H(func(c *zip.Ctx) error {
		sawPaths = append(sawPaths, c.Path())
		return c.Next()
	}))
	sub.Post("/deposit", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "deposit") })

	// A sibling NON-mint read, registered on the PARENT group. It must not be
	// touched by the sub-group's middleware.
	api.Get("/balance", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "balance") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/billing/balance", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/billing/balance: status=%d, want 200", resp.StatusCode)
	}

	for _, p := range sawPaths {
		if p == "/v1/billing/balance" {
			t.Fatalf("a sub-group's Use fired on the PARENT's sibling route %q — the leak is back.\n"+
				"    A gate on `api.Group(\"\")` would again cover ALL of /v1/billing, every org-admin\n"+
				"    read included. Mint's per-route gate is what makes that unrepresentable.", p)
		}
	}
	t.Logf("membership-scoped: the sub-group's middleware saw only its own routes (%v)", sawPaths)
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
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/v1/billing/mint-probe", nil))
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
	resp, err := app.Test(req)
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
