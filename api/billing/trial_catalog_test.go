package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/trial"
)

func TestEntryTrialPlanUsesCatalogEntryPlan(t *testing.T) {
	if trial.PlanSlug != "dev" {
		t.Fatalf("trial plan = %q, want catalog entry plan dev", trial.PlanSlug)
	}

	p := entryTrialPlan()
	if p.Slug != "dev" {
		t.Fatalf("entry trial plan = %q, want pro", p.Slug)
	}
	// Read from the catalog, not restated: the contract is that the trial charges
	// the SAME price the catalog publishes for the entry rung. Restating the number here just
	// means a reprice fails a test that has no opinion about the price.
	want := lookupPlan("dev")
	if want == nil {
		t.Fatal("catalog has no dev plan")
	}
	if p.PriceCents != want.Price {
		t.Fatalf("entry trial price = %d, want %d (the catalog's entry-rung price)", p.PriceCents, want.Price)
	}
	// The trial hands one month of the entry plan, so the credit IS that price.
	// Asserted as an equality rather than a number, so a reprice moves both and
	// fails neither. The `> 0` half is the load-bearing one: resolveEntryPlan
	// reads a zero credit as "trial not configured", so a trial funded from
	// anything the catalog can stop declaring switches itself off in silence.
	if p.CreditCents <= 0 {
		t.Fatalf("entry trial credit = %d, want a positive credit (zero reads as unconfigured)", p.CreditCents)
	}
	if p.CreditCents != want.Price {
		t.Fatalf("entry trial credit = %d, want %d (one month of the entry plan)", p.CreditCents, want.Price)
	}
}
