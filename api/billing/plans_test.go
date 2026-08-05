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
	must := []string{"go", "dev", "pro", "max", "team", "enterprise"}
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

	// `go` is the entry rung and carries the entry rate ceilings; `enterprise` is
	// priced by conversation, so it is the one row with a null price.
	entry := bySlug["go"]
	if entry.Price != 900 {
		t.Errorf("Go price = %d cents, want 900", entry.Price)
	}
	if entry.Limits == nil {
		t.Fatal("Go plan should have limits")
	}
	if entry.Limits.RequestsPerMinute == nil || *entry.Limits.RequestsPerMinute != 60 {
		t.Error("Go requestsPerMinute should be 60")
	}
	if entry.Limits.TokensPerMinute == nil || *entry.Limits.TokensPerMinute != 100000 {
		t.Error("Go tokensPerMinute should be 100000")
	}

	// The ladder climbs. Asserted as an ORDERING rather than four numbers, so it
	// keeps holding after a reprice and only fails on a rung that is out of order.
	prev := int64(-1)
	for _, slug := range []string{"go", "dev", "pro", "max"} {
		p := bySlug[slug]
		if p.Price <= prev {
			t.Errorf("ladder is not ascending at %q: %d cents follows %d", slug, p.Price, prev)
		}
		prev = p.Price
	}

	// Annual is a real discount on every rung, never merely equal to monthly —
	// which is what "no annual offer" looks like in this data.
	for _, slug := range []string{"go", "dev", "pro", "max", "team"} {
		p := bySlug[slug]
		if p.PriceAnnual <= 0 || p.PriceAnnual >= p.Price {
			t.Errorf("plan %q annual = %d cents/mo, monthly = %d; annual must be a discount", slug, p.PriceAnnual, p.Price)
		}
		// Stored per-month must multiply to a round annual total; otherwise the
		// page can advertise a yearly figure the stored number does not reach.
		if (p.PriceAnnual*12)%100 != 0 {
			t.Errorf("plan %q annual %d cents/mo x12 = %d cents, not a round dollar year", slug, p.PriceAnnual, p.PriceAnnual*12)
		}
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
	p := lookupPlan("go")
	if p == nil {
		t.Fatal("lookupPlan(go) returned nil")
	}
	if p.Slug != "go" {
		t.Errorf("lookupPlan(go).Slug = %q", p.Slug)
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

	// Contract MIRRORS the function's real precedence: the monthly allotment is
	// the plan's declared cloud-credit allowance, sourced (in order) from
	// limits.includedCloudCredits (flat), then includedCloudCreditsPerUser (per
	// seat), then the legacy includedCreditUsd alias — dollars*100. Whichever the
	// plan declares > 0, IncludedMonthlyCents returns it; otherwise 0. Asserting
	// the RELATIONSHIP (not a hard-coded number) keeps this stable across
	// @hanzo/plans catalog versions fetched at build time. (The prior test checked
	// only includedCreditUsd, so plans that declare includedCloudCredits — e.g.
	// max=$100, team-max=$100/seat — failed with "10000 want 0".)
	for _, p := range hanzoPlans {
		got := IncludedMonthlyCents(p.Slug)
		var want int64
		if p.Limits != nil {
			if usd := firstNonNilPositive(p.Limits.IncludedCloudCredits, p.Limits.IncludedCloudCreditsPerUser, p.Limits.IncludedCreditUsd); usd != nil {
				want = int64(*usd) * 100
			}
		}
		if got != want {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want %d (cloudCredits→cloudCreditsPerUser→creditUsd)", p.Slug, got, want)
		}
	}
}

// firstNonNilPositive returns the first argument that is non-nil AND > 0, or nil.
// Mirrors IncludedMonthlyCents's field-precedence selection so the test and the
// implementation cannot silently diverge.
func firstNonNilPositive(vals ...*int) *int {
	for _, v := range vals {
		if v != nil && *v > 0 {
			return v
		}
	}
	return nil
}
