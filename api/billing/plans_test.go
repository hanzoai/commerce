package billing

import (
	"testing"
)

func TestPlansLoaded(t *testing.T) {
	if len(hanzoPlans) == 0 {
		t.Fatal("hanzoPlans is empty — embedded subscription.json not loaded")
	}

	// Verify canonical plan count: Developer, Pro, Team, Enterprise, Custom.
	if got := len(hanzoPlans); got != 5 {
		t.Fatalf("expected 5 plans, got %d", got)
	}

	// Verify each plan has required fields.
	expectedSlugs := []string{"developer", "pro", "team", "enterprise", "custom"}
	for i, slug := range expectedSlugs {
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

	// Verify pricing in cents matches @hanzo/plans canonical values.
	// Developer: $0/mo
	if hanzoPlans[0].Price != 0 {
		t.Errorf("Developer price = %d cents, want 0", hanzoPlans[0].Price)
	}
	// Pro: $49/mo
	if hanzoPlans[1].Price != 4900 {
		t.Errorf("Pro price = %d cents, want 4900", hanzoPlans[1].Price)
	}
	// Pro annual: $39/mo
	if hanzoPlans[1].PriceAnnual != 3900 {
		t.Errorf("Pro annual = %d cents, want 3900", hanzoPlans[1].PriceAnnual)
	}
	// Team: $199/mo
	if hanzoPlans[2].Price != 19900 {
		t.Errorf("Team price = %d cents, want 19900", hanzoPlans[2].Price)
	}
	// Enterprise: $9999/mo
	if hanzoPlans[3].Price != 999900 {
		t.Errorf("Enterprise price = %d cents, want 999900", hanzoPlans[3].Price)
	}
	// Custom: contactSales=true
	if !hanzoPlans[4].ContactSales {
		t.Error("Custom plan should have contactSales=true")
	}

	// Verify limits are populated.
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
