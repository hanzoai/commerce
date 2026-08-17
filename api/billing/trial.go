package billing

import (
	"github.com/hanzoai/commerce/billing/trial"
)

// init wires the entry plan into billing/trial from the embedded plan catalog.
// The resolver is lazy, so it runs at request time (after all package init has
// loaded catalog) and billing/trial never imports the catalog — keeping the
// trial engine decomplected from plan economics.
func init() {
	trial.SetEntryPlanResolver(entryTrialPlan)
}

// entryTrialPlan projects the catalog's entry plan (trial.PlanSlug) onto the
// shape billing/trial needs. The trial credit is the plan's PRICE: it hands the
// customer one month of the entry plan up front, which is what a trial is.
// Reading an included allowance instead tied the trial to a monthly allotment
// the ladder no longer grants, which would have quietly resolved to zero and
// switched the trial off (resolveEntryPlan treats a zero credit as unconfigured).
func entryTrialPlan() trial.Plan {
	p := lookupPlan(trial.PlanSlug)
	if p == nil {
		return trial.Plan{}
	}
	credit := p.Price
	return trial.Plan{
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.Price,
		CreditCents: credit,
		Currency:    p.Currency,
	}
}
