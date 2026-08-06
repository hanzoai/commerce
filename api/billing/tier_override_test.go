// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/zap-proto/zip"
)

// resolveOverride drives resolveTierName through a real zip request and reports the
// tier it settled on. No org is seeded, so the DERIVED answer is Free — which makes
// "did the override win?" unambiguous: Free means it was ignored.
func resolveOverride(t *testing.T, req *http.Request) tier.Name {
	t.Helper()
	var got tier.Name
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.All("/t", func(c *zip.Ctx) error {
		n, err := resolveTierName(c, "alice")
		if err != nil {
			return err
		}
		got = n
		return c.JSON(200, map[string]any{"ok": true})
	})
	if _, err := app.Test(req); err != nil {
		t.Fatalf("drive: %v", err)
	}
	return got
}

// A TIER OVERRIDE IS A MINT, so an unprivileged caller must not be able to use one.
//
// Both sources are CLIENT INPUT. `?tier=` obviously; X-Tier too — the gateway
// neither mints that header (iamauth.MintedIdentityHeaders) nor strips it
// (StripIdentityHeaderNames), so it arrives from whoever sent the request despite a
// comment calling it authoritative. Measured live before this clamp:
//
//	X-Tier: enterprise  -> enterprise, unlimitedAgents true
//	?tier=max           -> pro, allowedModels ["*"], maxAgents 10
//
// A tier decides which models a caller may invoke and how many agents it may run,
// so granting one is minting.
func TestTierOverrideIsIgnoredWithoutMintAuthority(t *testing.T) {
	for _, override := range []string{"enterprise", "max", "pro", "team", "starter"} {
		req := httptest.NewRequest("GET", "/t?tier="+override, nil)
		if got := resolveOverride(t, req); got != tier.Free {
			t.Errorf("?tier=%s without mint authority resolved to %q — an unprivileged "+
				"caller named its own entitlement", override, got)
		}
	}
	for _, override := range []string{"enterprise", "pro", "max"} {
		req := httptest.NewRequest("GET", "/t", nil)
		req.Header.Set(iammiddleware.HeaderTier, override)
		if got := resolveOverride(t, req); got != tier.Free {
			t.Errorf("X-Tier: %s without mint authority resolved to %q — that header is "+
				"neither minted nor stripped by the gateway, so it is client input",
				override, got)
		}
	}
}

// The clamp must not break the LEGITIMATE override — an operator and the S2S
// readers still name a tier. Mirrors planForGrant, which honours an explicit plan
// for exactly this predicate.
func TestTierOverrideIsHonouredWithMintAuthority(t *testing.T) {
	req := httptest.NewRequest("GET", "/t?tier=enterprise", nil)
	req.Header.Set(iammiddleware.HeaderUserOwner, "admin") // IsSuperAdmin -> MayMintMoney
	if got := resolveOverride(t, req); got != tier.Enterprise {
		t.Errorf("a mint-authorised ?tier=enterprise resolved to %q, want enterprise — "+
			"the clamp must not break the legitimate override", got)
	}

	req = httptest.NewRequest("GET", "/t", nil)
	req.Header.Set(iammiddleware.HeaderUserOwner, "admin")
	req.Header.Set(iammiddleware.HeaderTier, "pro")
	if got := resolveOverride(t, req); got != tier.Pro {
		t.Errorf("a mint-authorised X-Tier: pro resolved to %q, want pro", got)
	}
}

// Both sources are treated identically — which one arrived must not change how much
// it is trusted.
func TestFirstOverrideTreatsBothSourcesEqually(t *testing.T) {
	if got := firstOverride("  ", " max "); got != "max" {
		t.Errorf("firstOverride(%q,%q) = %q, want max", "  ", " max ", got)
	}
	if got := firstOverride("enterprise", "max"); got != "enterprise" {
		t.Errorf("firstOverride = %q, want the first non-empty", got)
	}
	if got := firstOverride("", "", "  "); got != "" {
		t.Errorf("firstOverride = %q, want empty", got)
	}
}
