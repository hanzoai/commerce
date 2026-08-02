package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/trial"
)

func TestEntryTrialPlanUsesCatalogPro(t *testing.T) {
	if trial.PlanSlug != "pro" {
		t.Fatalf("trial plan = %q, want catalog plan pro", trial.PlanSlug)
	}

	p := entryTrialPlan()
	if p.Slug != "pro" {
		t.Fatalf("entry trial plan = %q, want pro", p.Slug)
	}
	// Read from the catalog, not restated: the contract is that the trial charges
	// the SAME price the catalog publishes for pro. Restating the number here just
	// means a reprice fails a test that has no opinion about the price.
	want := lookupPlan("pro")
	if want == nil {
		t.Fatal("catalog has no pro plan")
	}
	if p.PriceCents != want.Price {
		t.Fatalf("entry trial price = %d, want %d (the catalog's pro price)", p.PriceCents, want.Price)
	}
	if p.CreditCents <= 0 {
		t.Fatalf("entry trial credit = %d, want positive catalog allowance", p.CreditCents)
	}
}
