// Package credit holds the canonical starter/welcome-credit policy VALUES.
//
// It is data-only: the constants below are the policy a caller passes to the ONE
// credit primitive, POST /v1/billing/credit (api/billing.Credit). "Granting a
// starter credit" is therefore just that endpoint called with tag=StarterCreditTag,
// amountCents=StarterCreditCents, expiresAt=now+StarterCreditDays — there is no
// second granting code path here (that was the point of the consolidation).
package credit

// Starter credit constants.
//
// The welcome/starter credit is a TRIAL (non-cash) grant: $5.00, classified by
// its `starter-credit` tag into the Credit bucket (billing/bucket.DepositKind),
// spendable on non-premium metered usage only (never GPUs, never premium models —
// see hanzoai/ai openai_api.go, which gates premium behind a balance ABOVE the
// $5 starter). $5 is the ONE canonical amount, shared across the stack: billing
// app config `trialCreditCents: 500`, ai `StarterCreditDollars = 5.00`, and the
// cloud-api-models `features.starter_credit: 5.0`. Keep them equal.
const (
	StarterCreditCents = 500 // $5.00 USD
	StarterCreditDays  = 365 // expires in 365 days
	StarterCreditTag   = "starter-credit"
)
