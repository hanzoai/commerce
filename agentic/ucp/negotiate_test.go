// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"reflect"
	"testing"
)

func caps(m map[string][]string, extends map[string]json_) map[string][]Capability {
	out := map[string][]Capability{}
	for name, versions := range m {
		for _, v := range versions {
			c := Capability{Entity: Entity{Version: v}}
			if e, ok := extends[name]; ok {
				c.Extends = e.raw()
			}
			out[name] = append(out[name], c)
		}
	}
	return out
}

type json_ []string

func (j json_) raw() []byte { return Extends([]string(j)...) }

func names(a *Active) []string { return a.Names() }

const (
	checkout    = "dev.ucp.shopping.checkout"
	fulfillment = "dev.ucp.shopping.fulfillment"
	discount    = "dev.ucp.shopping.discount"
	cart        = "dev.ucp.shopping.cart"
	loyalty     = "com.example.loyalty"
)

func TestNegotiationIntersectsByName(t *testing.T) {
	biz := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		checkout: {"2026-08-25"}, fulfillment: {"2026-08-25"}, discount: {"2026-08-25"},
	}, map[string]json_{fulfillment: {checkout}, discount: {checkout, cart}})}
	plat := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		checkout: {"2026-08-25"}, fulfillment: {"2026-08-25"}, "dev.ucp.shopping.order": {"2026-08-25"},
	}, nil)}
	got := names(Negotiate(biz, plat, nil))
	if want := []string{checkout, fulfillment}; !reflect.DeepEqual(got, want) {
		t.Fatalf("active = %v; want %v (discount is the business's only, order the platform's only)", got, want)
	}
}

func TestNegotiationSelectsTheHighestSharedVersion(t *testing.T) {
	biz := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{loyalty: {"2026-01-01", "2026-06-15", "2026-09-01"}}, nil)}
	plat := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{loyalty: {"2026-01-01", "2026-06-15", "2026-07-01"}}, nil)}
	a := Negotiate(biz, plat, nil)
	if a.Capabilities[loyalty].Version != "2026-06-15" {
		t.Fatalf("selected %q; want the highest version in both arrays", a.Capabilities[loyalty].Version)
	}
	// Compatibility is exact version equality, never inferred from date order.
	plat.Capabilities = caps(map[string][]string{loyalty: {"2026-07-01"}}, nil)
	if a := Negotiate(biz, plat, nil); len(a.Capabilities) != 0 {
		t.Fatalf("active = %v; no shared version excludes the capability", names(a))
	}
}

func TestOrphanedExtensionsArePrunedTransitively(t *testing.T) {
	biz := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		checkout: {"2026-08-25"}, fulfillment: {"2026-08-25"}, "com.example.pickup": {"2026-08-25"}, discount: {"2026-08-25"},
	}, map[string]json_{fulfillment: {checkout}, "com.example.pickup": {fulfillment}, discount: {checkout, cart}})}
	// The platform does not declare checkout: every dependent goes, transitively.
	plat := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		fulfillment: {"2026-08-25"}, "com.example.pickup": {"2026-08-25"}, discount: {"2026-08-25"},
	}, nil)}
	if a := Negotiate(biz, plat, nil); len(a.Capabilities) != 0 {
		t.Fatalf("active = %v", names(a))
	}
	// A multi-parent extension survives on any one parent.
	plat.Capabilities = caps(map[string][]string{cart: {"2026-08-25"}, discount: {"2026-08-25"}}, nil)
	biz.Capabilities[cart] = caps(map[string][]string{cart: {"2026-08-25"}}, nil)[cart]
	if got, want := names(Negotiate(biz, plat, nil)), []string{cart, discount}; !reflect.DeepEqual(got, want) {
		t.Fatalf("active = %v; want %v", got, want)
	}
}

func TestExtensionRequirementsExcludeAndRePrune(t *testing.T) {
	biz := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		checkout: {"2026-08-25"}, loyalty: {"2026-06-15"}, "com.example.tier": {"2026-06-15"},
	}, map[string]json_{loyalty: {checkout}, "com.example.tier": {loyalty}})}
	plat := &Meta{Version: "2026-08-25", Capabilities: caps(map[string][]string{
		checkout: {"2026-08-25"}, loyalty: {"2026-06-15"}, "com.example.tier": {"2026-06-15"},
	}, nil)}
	req := map[string]Requires{loyalty: {
		Protocol:     &VersionRange{Min: "2026-01-23"},
		Capabilities: map[string]VersionRange{checkout: {Min: "2026-06-01"}},
	}}
	if got, want := names(Negotiate(biz, plat, req)), []string{loyalty, "com.example.tier", checkout}; !reflect.DeepEqual(got, want) {
		t.Fatalf("active = %v; want %v", got, want)
	}
	req[loyalty] = Requires{Capabilities: map[string]VersionRange{checkout: {Min: "2026-09-01"}}}
	if got, want := names(Negotiate(biz, plat, req)), []string{checkout}; !reflect.DeepEqual(got, want) {
		t.Fatalf("active = %v; want %v (loyalty excluded, its dependent re-pruned)", got, want)
	}
	req[loyalty] = Requires{Protocol: &VersionRange{Min: "2026-01-01", Max: "2026-08-01"}}
	if got := names(Negotiate(biz, plat, req)); !reflect.DeepEqual(got, []string{checkout}) {
		t.Fatalf("a protocol range that excludes the selected version: %v", got)
	}
}

func TestVersionRangeIsInclusive(t *testing.T) {
	r := VersionRange{Min: "2026-01-23", Max: "2026-09-01"}
	for v, want := range map[string]bool{"2026-01-22": false, "2026-01-23": true, "2026-09-01": true, "2026-09-02": false} {
		if r.Has(v) != want {
			t.Errorf("Has(%s) = %v", v, !want)
		}
	}
	if !(VersionRange{Min: "2026-01-23"}).Has("2999-01-01") {
		t.Error("no max means no upper bound")
	}
}

// Response Capability Selection: the table of the overview.
func TestRelevantSelectsCapabilitiesByOperation(t *testing.T) {
	a := &Active{Version: Version, Capabilities: map[string]Capability{
		checkout:    {Entity: Entity{Version: Version}},
		fulfillment: {Entity: Entity{Version: Version}, Extends: Extends(checkout)},
		discount:    {Entity: Entity{Version: Version}, Extends: Extends(checkout, cart)},
		cart:        {Entity: Entity{Version: Version}},
		CapOrder:    {Entity: Entity{Version: Version}},
	}}
	keys := func(m map[string][]Capability) []string { return sortedKeys(m) }
	if got, want := keys(a.Relevant(checkout)), []string{checkout, discount, fulfillment}; !reflect.DeepEqual(got, want) {
		t.Errorf("checkout response = %v; want %v", got, want)
	}
	if got, want := keys(a.Relevant(cart)), []string{cart, discount}; !reflect.DeepEqual(got, want) {
		t.Errorf("cart response = %v; want %v", got, want)
	}
	if got, want := keys(a.Relevant(CapOrder)), []string{CapOrder}; !reflect.DeepEqual(got, want) {
		t.Errorf("order response = %v; want %v", got, want)
	}
	if len(a.Relevant("dev.ucp.shopping.catalog")) != 0 {
		t.Error("a root that was not negotiated has no capabilities")
	}
	if e := a.Relevant(checkout)[checkout]; len(e) != 1 || e[0].Version != Version || e[0].Schema != "" {
		t.Errorf("a response entry carries the version only: %+v", e)
	}
}
