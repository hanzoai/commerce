package billing

import (
	"github.com/hanzoai/commerce/billing/trial"
)

// init wires the $20/mo Pro plan into billing/trial from the embedded plan
// catalog. The resolver is lazy, so it runs at request time (after all package
// init has loaded hanzoPlans) and billing/trial never imports the catalog —
// keeping the trial engine decomplected from plan economics.
func init() {
	trial.SetEntryPlanResolver(entryTrialPlan)
}

// entryTrialPlan projects the catalog's entry plan (trial.PlanSlug) onto the
// shape billing/trial needs. The unified trial credit is funded from the plan's
// monthly included allowance (limits.includedCreditUsd), so the trial hands the
// customer exactly one month of the $20 plan's usage up front.
func entryTrialPlan() trial.Plan {
	p := lookupPlan(trial.PlanSlug)
	if p == nil {
		return trial.Plan{}
	}
	credit := IncludedMonthlyCents(p.Slug)
	if p.Limits != nil && p.Limits.IncludedCreditUsd != nil {
		credit = int64(*p.Limits.IncludedCreditUsd) * 100
	}
	return trial.Plan{
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.Price,
		CreditCents: credit,
		Currency:    p.Currency,
	}
}
