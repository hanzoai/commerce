// Package seed — one-shot seed helpers for local dev.
//
// Nothing in this package runs automatically; callers must opt in by (a)
// importing it and (b) setting COMMERCE_SEED_LIQUIDITY_DEV=1. Production
// deployments MUST NOT run seeds — tenant rows in prod are created through
// the admin API at deploy time, with full audit trail.
package seed

import (
	"errors"
	"fmt"
	"os"

	"github.com/hanzoai/commerce/store"
)

// SeedLiquidityDev creates the Liquidity tenant row with devnet hostnames
// iff the gate env var is set. It is idempotent: a second call when the
// tenant already exists returns nil.
//
// Usage (from commerced main, only after store.New succeeds):
//
//	if err := seed.SeedLiquidityDev(s); err != nil {
//	    log.Fatal(err)
//	}
func SeedLiquidityDev(s *store.Store) error {
	if os.Getenv("COMMERCE_SEED_LIQUIDITY_DEV") != "1" {
		return nil // gate closed — no-op
	}
	if s == nil || s.Tenants == nil {
		return errors.New("seed: nil store")
	}

	// Check for an existing Liquidity tenant and short-circuit if found.
	tenants, err := s.Tenants.List(500, 0)
	if err != nil {
		return fmt.Errorf("seed: list tenants: %w", err)
	}
	for _, t := range tenants {
		if t.Name == "liquidity" {
			return nil // already seeded
		}
	}

	t := &store.Tenant{
		Name: "liquidity",
		Hostnames: []string{
			"pay.dev.",
			"pay.test.",
			"pay.dev.",
			"pay.test.",
		},
		Brand: store.BrandConfig{
			DisplayName:  "",
			LogoURL:      "https://cdn./liquidity.png",
			PrimaryColor: "#0ea5e9",
		},
		IAM: store.IAMConfig{
			Issuer:   "https://id.",
			ClientID: "liquidity-exchange-client-id",
		},
		IDV: store.IDVConfig{
			Provider: "persona",
			Endpoint: "https://withpersona.com/verify",
		},
		// No credentials in this row — providers[].kms_path references
		// are set by the credential-upload admin flow (future slice).
		Providers: []store.Provider{
			{Name: "square", Enabled: true, KMSPath: "commerce/liquidity/square"},
			{Name: "stripe", Enabled: false, KMSPath: "commerce/liquidity/stripe"},
		},
		BDEndpoint: "https://bd.dev.",
		ReturnURLAllowlist: []string{
			"https://dev.",
			"https://test.",
			"https://dev.",
			"https://test.",
		},
	}

	if err := s.Tenants.Create(t); err != nil {
		return fmt.Errorf("seed: create liquidity: %w", err)
	}
	return nil
}
