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
	sub.Raw(http.MethodPost, "/deposit", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "deposit") })

	// A sibling NON-mint read, registered on the PARENT group. It must not be
	// touched by the sub-group's middleware.
	api.Raw(http.MethodGet, "/balance", func(c *zip.Ctx) error { return c.JSON(http.StatusOK, "balance") })

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

// Mint gates raw and typed routes, and leaves the parent's routes open.
func TestMintGatesEveryRouteBeneathIt(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	billing := app.Group("/v1").Group("billing")
	mint := Mint(billing, "/v1/billing")

	reached := false
	mint.Raw(http.MethodPost, "/mint-probe", func(c *zip.Ctx) error {
		reached = true
		return c.JSON(http.StatusOK, "minted")
	})

	type depositIn struct {
		Cents int `json:"cents"`
	}
	type depositOut struct {
		OK bool `json:"ok"`
	}
	mint.Post("/deposit", func(context.Context, *depositIn) (*depositOut, error) {
		return &depositOut{OK: true}, nil
	})

	open := false
	billing.Raw(http.MethodGet, "/balance", func(c *zip.Ctx) error {
		open = true
		return c.JSON(http.StatusOK, "balance")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/v1/billing/mint-probe", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("raw mint route answered %d, want 403", resp.StatusCode)
	}
	if reached {
		t.Fatal("handler ran despite the gate")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/deposit", strings.NewReader(`{"cents":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("typed mint op answered %d — the gate did not run", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/v1/billing/balance", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !open {
		t.Fatalf("the parent's read answered %d — the gate reached a route that mints nothing", resp.StatusCode)
	}

	if len(app.Commands()) != 1 || app.Commands()[0].Path != "/v1/billing/deposit" {
		t.Fatalf("commands = %+v, want the one op", app.Commands())
	}
	if len(app.MCPTools()) != 1 {
		t.Fatalf("tools = %+v, want the one op", app.MCPTools())
	}
}
