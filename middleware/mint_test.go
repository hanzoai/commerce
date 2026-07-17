// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"net/http/httptest"
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
// This test PROVES the leak: a marker middleware on a bare sub-group fires on a
// sibling route registered against the PARENT. That is why Mint prepends the
// gate per-route instead, and why mintRouter.Use is refused outright.
func TestGroupUseIsPrefixScopedNotMembership(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	api := app.Group("/v1").Group("billing")

	// A bare sub-group of api with its own middleware — the "authz is middleware
	// on a group" shape.
	sub := api.Group("")
	var sawPaths []string
	sub.Use(func(c *zip.Ctx) error {
		sawPaths = append(sawPaths, c.Path())
		return c.Next()
	})
	sub.Post("/deposit", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "deposit") })

	// A sibling NON-mint read, registered on the PARENT group. It must not be
	// touched by the sub-group's middleware.
	api.Get("/balance", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "balance") })

	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/v1/billing/balance", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/billing/balance: status=%d, want 200", resp.StatusCode)
	}

	for _, p := range sawPaths {
		if p == "/v1/billing/balance" {
			t.Logf("CONFIRMED: a bare sub-group's Use fired on the PARENT's sibling route %q.", p)
			t.Logf("  fiber Use is prefix-scoped, so `api.Group(\"\").Use(gate)` gates ALL of /v1/billing —")
			t.Logf("  every org-admin read included. Mint therefore prepends the gate per-route.")
			return
		}
	}
	t.Fatalf("bare sub-group Use did NOT fire on the parent's sibling route (saw %v).\n"+
		"    fiber's Use scoping may have changed: if Use is now membership-scoped, Mint SHOULD be\n"+
		"    rewritten as a gated sub-group (mint := api.Group(\"\"); mint.Use(PlatformOnly())) — the\n"+
		"    simpler shape this test exists to rule out.", sawPaths)
}

// TestMintRecordsFullPathAndGates proves the two halves of the single
// declaration: the route is recorded with the full path fiber routes on, and the
// gate is really in its chain (an ungated caller is refused before the handler).
func TestMintRecordsFullPathAndGates(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	app := zip.New(zip.Config{DisableStartupMessage: true})
	api := app.Group("/v1").Group("billing")

	reached := false
	Mint(api).Post("/mint-probe", func(c *zip.Ctx) error {
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
		Mint(app.Group("/v1").Group("billing")).Post("/set-probe", func(c *zip.Ctx) error { return nil })
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
	Mint(app.Group("/v1").Group("billing")).Use(func(c *zip.Ctx) error { return nil })
}
