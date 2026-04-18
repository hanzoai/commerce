package billing

import (
	"testing"
)

func TestPlansLoaded(t *testing.T) {
	if len(hanzoPlans) == 0 {
		t.Fatal("hanzoPlans is empty")
	}

	// Build a slug → plan index so additions to the catalog (e.g. Team Max,
	// World tiers) do not break positional assumptions.
	bySlug := make(map[string]staticPlan, len(hanzoPlans))
	for _, p := range hanzoPlans {
		bySlug[p.Slug] = p
	}

	// Core subscription plans that must exist.
	must := []string{"developer", "pro", "team", "enterprise"}
	for _, slug := range must {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("required plan %q missing", slug)
		}
		if p.Name == "" {
			t.Errorf("plan %q.Name is empty", slug)
		}
		if p.Currency != "usd" {
			t.Errorf("plan %q.Currency = %q, want usd", slug, p.Currency)
		}
	}

	// World plans added for Hanzo World.
	for _, slug := range []string{"world-free", "world-pro", "world-team"} {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("required world plan %q missing", slug)
		}
		if p.Category != "world" {
			t.Errorf("plan %q.Category = %q, want world", slug, p.Category)
		}
	}

	// Developer pricing and limits — these are spec-level invariants we do
	// check precisely.
	dev := bySlug["developer"]
	if dev.Price != 0 {
		t.Errorf("Developer price = %d cents, want 0", dev.Price)
	}
	if dev.Limits == nil {
		t.Fatal("Developer plan should have limits")
	}
	if dev.Limits.RequestsPerMinute == nil || *dev.Limits.RequestsPerMinute != 60 {
		t.Error("Developer requestsPerMinute should be 60")
	}
	if dev.Limits.TokensPerMinute == nil || *dev.Limits.TokensPerMinute != 100000 {
		t.Error("Developer tokensPerMinute should be 100000")
	}

	// Enterprise is contact-sales.
	if !bySlug["enterprise"].ContactSales {
		t.Error("Enterprise plan should have contactSales=true")
	}

	// World Pro has a non-zero price and is marked popular.
	wp := bySlug["world-pro"]
	if wp.Price != 2900 {
		t.Errorf("world-pro price = %d cents, want 2900", wp.Price)
	}
	if !wp.Popular {
		t.Error("world-pro should be marked popular")
	}
}

func TestDNSPlansLoaded(t *testing.T) {
	if len(dnsPlans) == 0 {
		t.Fatal("dnsPlans is empty")
	}

	if got := len(dnsPlans); got != 3 {
		t.Fatalf("expected 3 DNS plans, got %d", got)
	}

	expectedSlugs := []string{"dns-free", "dns-pro", "dns-enterprise"}
	for i, slug := range expectedSlugs {
		if dnsPlans[i].Slug != slug {
			t.Errorf("dnsPlans[%d].Slug = %q, want %q", i, dnsPlans[i].Slug, slug)
		}
		if dnsPlans[i].Category != "dns" {
			t.Errorf("dnsPlans[%d].Category = %q, want %q", i, dnsPlans[i].Category, "dns")
		}
		if dnsPlans[i].Currency != "usd" {
			t.Errorf("dnsPlans[%d].Currency = %q, want %q", i, dnsPlans[i].Currency, "usd")
		}
		if dnsPlans[i].Name == "" {
			t.Errorf("dnsPlans[%d].Name is empty", i)
		}
		if len(dnsPlans[i].Features) == 0 {
			t.Errorf("dnsPlans[%d].Features is empty", i)
		}
	}

	// DNS Free: $0/mo
	if dnsPlans[0].Price != 0 {
		t.Errorf("DNS Free price = %d cents, want 0", dnsPlans[0].Price)
	}
	// DNS Pro: $5/mo
	if dnsPlans[1].Price != 500 {
		t.Errorf("DNS Pro price = %d cents, want 500", dnsPlans[1].Price)
	}
	// DNS Pro annual: $4/mo
	if dnsPlans[1].PriceAnnual != 400 {
		t.Errorf("DNS Pro annual = %d cents, want 400", dnsPlans[1].PriceAnnual)
	}
	// DNS Enterprise: $25/mo
	if dnsPlans[2].Price != 2500 {
		t.Errorf("DNS Enterprise price = %d cents, want 2500", dnsPlans[2].Price)
	}
	// DNS Pro should be popular
	if !dnsPlans[1].Popular {
		t.Error("DNS Pro plan should be popular")
	}

	// Verify DNS limits.
	if dnsPlans[0].Limits == nil {
		t.Fatal("DNS Free plan should have limits")
	}
	if dnsPlans[0].Limits.Zones == nil || *dnsPlans[0].Limits.Zones != 2 {
		t.Error("DNS Free zones should be 2")
	}
	if dnsPlans[0].Limits.RecordsPerZone == nil || *dnsPlans[0].Limits.RecordsPerZone != 50 {
		t.Error("DNS Free recordsPerZone should be 50")
	}
	if dnsPlans[0].Limits.QueriesPerDay == nil || *dnsPlans[0].Limits.QueriesPerDay != 10000 {
		t.Error("DNS Free queriesPerDay should be 10000")
	}

	if dnsPlans[1].Limits == nil {
		t.Fatal("DNS Pro plan should have limits")
	}
	if dnsPlans[1].Limits.Zones == nil || *dnsPlans[1].Limits.Zones != 25 {
		t.Error("DNS Pro zones should be 25")
	}
	if dnsPlans[1].Limits.RecordsPerZone == nil || *dnsPlans[1].Limits.RecordsPerZone != 500 {
		t.Error("DNS Pro recordsPerZone should be 500")
	}
	if dnsPlans[1].Limits.QueriesPerDay == nil || *dnsPlans[1].Limits.QueriesPerDay != 1000000 {
		t.Error("DNS Pro queriesPerDay should be 1000000")
	}

	if dnsPlans[2].Limits == nil {
		t.Fatal("DNS Enterprise plan should have limits")
	}
	if dnsPlans[2].Limits.Zones == nil || *dnsPlans[2].Limits.Zones != -1 {
		t.Error("DNS Enterprise zones should be -1 (unlimited)")
	}
	if dnsPlans[2].Limits.QueriesPerDay == nil || *dnsPlans[2].Limits.QueriesPerDay != -1 {
		t.Error("DNS Enterprise queriesPerDay should be -1 (unlimited)")
	}
}

func TestLookupPlan(t *testing.T) {
	p := lookupPlan("developer")
	if p == nil {
		t.Fatal("lookupPlan(developer) returned nil")
	}
	if p.Slug != "developer" {
		t.Errorf("lookupPlan(developer).Slug = %q", p.Slug)
	}

	p = lookupPlan("dns-pro")
	if p == nil {
		t.Fatal("lookupPlan(dns-pro) returned nil")
	}
	if p.Slug != "dns-pro" {
		t.Errorf("lookupPlan(dns-pro).Slug = %q", p.Slug)
	}

	p = lookupPlan("nonexistent-plan")
	if p != nil {
		t.Errorf("lookupPlan(nonexistent-plan) should return nil, got %v", p.Slug)
	}
}
