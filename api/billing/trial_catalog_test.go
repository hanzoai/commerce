package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/trial"
)

func TestEntryTrialPlanUsesCatalogPro(t *testing.T) {
	if trial.PlanSlug != "pro" {
		t.Fatalf("trial plan = %q, want catalog $20 plan pro", trial.PlanSlug)
	}

	p := entryTrialPlan()
	if p.Slug != "pro" {
		t.Fatalf("entry trial plan = %q, want pro", p.Slug)
	}
	if p.PriceCents != 2000 {
		t.Fatalf("entry trial price = %d, want 2000 cents", p.PriceCents)
	}
	if p.CreditCents <= 0 {
		t.Fatalf("entry trial credit = %d, want positive catalog allowance", p.CreditCents)
	}
}
