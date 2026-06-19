package billing

import (
	"testing"
)

// indexBySlug builds a slug → plan map so tests never depend on the positional
// order of the embedded catalog (which grows as @hanzo/plans adds tiers).
func indexBySlug(plans []staticPlan) map[string]staticPlan {
	m := make(map[string]staticPlan, len(plans))
	for _, p := range plans {
		m[p.Slug] = p
	}
	return m
}

func TestPlansLoaded(t *testing.T) {
	if len(hanzoPlans) == 0 {
		t.Fatal("hanzoPlans is empty")
	}

	bySlug := indexBySlug(hanzoPlans)

	// Core subscription plans that must exist regardless of catalog growth.
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

	// Developer is free; Enterprise is a real paid plan; both have limits.
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
}

func TestDNSPlansLoaded(t *testing.T) {
	if len(dnsPlans) == 0 {
		t.Fatal("dnsPlans is empty")
	}

	if got := len(dnsPlans); got != 3 {
		t.Fatalf("expected 3 DNS plans, got %d", got)
	}

	bySlug := indexBySlug(dnsPlans)
	for _, slug := range []string{"dns-free", "dns-pro", "dns-enterprise"} {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("required DNS plan %q missing", slug)
		}
		if p.Category != "dns" {
			t.Errorf("DNS plan %q.Category = %q, want dns", slug, p.Category)
		}
		if p.Currency != "usd" {
			t.Errorf("DNS plan %q.Currency = %q, want usd", slug, p.Currency)
		}
		if len(p.Features) == 0 {
			t.Errorf("DNS plan %q.Features is empty", slug)
		}
		if p.Limits == nil {
			t.Fatalf("DNS plan %q should have limits", slug)
		}
	}

	if c := bySlug["dns-free"].Price; c != 0 {
		t.Errorf("DNS Free price = %d cents, want 0", c)
	}
	if !bySlug["dns-pro"].Popular {
		t.Error("DNS Pro plan should be popular")
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

func TestIncludedMonthlyCents(t *testing.T) {
	// Unknown plan -> 0 (no allotment).
	if c := IncludedMonthlyCents("nonexistent-plan"); c != 0 {
		t.Errorf("IncludedMonthlyCents(unknown) = %d, want 0", c)
	}

	// Contract: whenever a loaded plan declares limits.includedCreditUsd > 0,
	// IncludedMonthlyCents returns dollars*100; otherwise 0. Asserting the
	// relationship (not a hard-coded number) keeps the test stable across
	// @hanzo/plans catalog versions fetched at build time.
	for _, p := range hanzoPlans {
		got := IncludedMonthlyCents(p.Slug)
		if p.Limits != nil && p.Limits.IncludedCreditUsd != nil && *p.Limits.IncludedCreditUsd > 0 {
			want := int64(*p.Limits.IncludedCreditUsd) * 100
			if got != want {
				t.Errorf("IncludedMonthlyCents(%q) = %d, want %d", p.Slug, got, want)
			}
		} else if got != 0 {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want 0 (no included allotment)", p.Slug, got)
		}
	}
}
