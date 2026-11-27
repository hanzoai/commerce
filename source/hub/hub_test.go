// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub_test

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/source/hub"
)

func TestSealAndOpenCredentialsBoundToOrgAndSource(t *testing.T) {
	ctx := context.Background()
	sealed, err := hub.SealCredentials(ctx, "carbon-district", "shopify_us", hub.SourceCredentials{
		AccessToken: "shpat_live_secret_token",
		HMACSecret:  "6b1f0a3c9e2d4b5a8c7f6e5d4c3b2a1908f7e6d5c4b3a2918f7e6d5c4b3a2918",
	})
	if err != nil {
		t.Fatalf("SealCredentials: %v", err)
	}
	opened, err := hub.OpenCredentials(ctx, "carbon-district", "shopify_us", sealed)
	if err != nil {
		t.Fatalf("OpenCredentials: %v", err)
	}
	if opened.AccessToken != "shpat_live_secret_token" {
		t.Fatalf("unexpected AccessToken: %q", opened.AccessToken)
	}
	// Cross-org or cross-source unseal MUST fail closed (AAD + HKDF binding).
	if _, err := hub.OpenCredentials(ctx, "other-org", "shopify_us", sealed); err == nil {
		t.Fatal("OpenCredentials succeeded across org boundary")
	}
	if _, err := hub.OpenCredentials(ctx, "carbon-district", "shopify_eu", sealed); err == nil {
		t.Fatal("OpenCredentials succeeded across source_id boundary")
	}
}

func TestHashProductOrderIndependence(t *testing.T) {
	a := hub.CanonicalProduct{
		SKU:         "TEE",
		Name:        "Carbon Tee",
		Currency:    "USD",
		PriceCents:  4500,
		Available:   true,
		ManageStock: true,
		Inventory:   15,
		Variants: []hub.CanonicalVariant{
			{SKU: "TEE-M", PriceCents: 4500, Available: true, ManageStock: true, Inventory: 10},
			{SKU: "TEE-S", PriceCents: 4500, Available: true, ManageStock: true, Inventory: 5},
		},
	}
	b := a
	b.Variants = []hub.CanonicalVariant{a.Variants[1], a.Variants[0]}
	b.ExternalId = "different-external-id"
	if hub.HashProduct(a) != hub.HashProduct(b) {
		t.Fatal("HashProduct must be invariant to variant slice order and external IDs")
	}
}
