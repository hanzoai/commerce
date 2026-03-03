package billing

import (
	"testing"
)

func TestPlansLoaded(t *testing.T) {
	if len(hanzoPlans) == 0 {
		t.Fatal("hanzoPlans is empty")
	}

	// Total plans: 5 subscription + 3 DNS = 8.
	if got := len(hanzoPlans); got != 8 {
		t.Fatalf("expected 8 total plans, got %d", got)
	}

	// Verify subscription plans (first 5).
	expectedSubSlugs := []string{"developer", "pro", "team", "enterprise", "custom"}
	for i, slug := range expectedSubSlugs {
		if hanzoPlans[i].Slug != slug {
			t.Errorf("plan[%d].Slug = %q, want %q", i, hanzoPlans[i].Slug, slug)
		}
		if hanzoPlans[i].Name == "" {
			t.Errorf("plan[%d].Name is empty", i)
		}
		if hanzoPlans[i].Currency != "usd" {
			t.Errorf("plan[%d].Currency = %q, want %q", i, hanzoPlans[i].Currency, "usd")
		}
	}

	// Verify subscription pricing in cents.
	if hanzoPlans[0].Price != 0 {
		t.Errorf("Developer price = %d cents, want 0", hanzoPlans[0].Price)
	}
	if hanzoPlans[1].Price != 4900 {
		t.Errorf("Pro price = %d cents, want 4900", hanzoPlans[1].Price)
	}
	if hanzoPlans[1].PriceAnnual != 3900 {
		t.Errorf("Pro annual = %d cents, want 3900", hanzoPlans[1].PriceAnnual)
	}
	if hanzoPlans[2].Price != 19900 {
		t.Errorf("Team price = %d cents, want 19900", hanzoPlans[2].Price)
	}
	if hanzoPlans[3].Price != 999900 {
		t.Errorf("Enterprise price = %d cents, want 999900", hanzoPlans[3].Price)
	}
	if !hanzoPlans[4].ContactSales {
		t.Error("Custom plan should have contactSales=true")
	}

	// Verify subscription limits are populated.
	if hanzoPlans[0].Limits == nil {
		t.Fatal("Developer plan should have limits")
	}
	if hanzoPlans[0].Limits.RequestsPerMinute == nil || *hanzoPlans[0].Limits.RequestsPerMinute != 60 {
		t.Error("Developer requestsPerMinute should be 60")
	}
	if hanzoPlans[0].Limits.TokensPerMinute == nil || *hanzoPlans[0].Limits.TokensPerMinute != 100000 {
		t.Error("Developer tokensPerMinute should be 100000")
	}

	// Verify features are populated.
	if len(hanzoPlans[0].Features) == 0 {
		t.Error("Developer plan should have features")
	}
	if len(hanzoPlans[1].Features) == 0 {
		t.Error("Pro plan should have features")
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
