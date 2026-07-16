// Package credit holds the shared constants for the one-time starter/welcome
// credit. It is deliberately DATA-ONLY: there is exactly ONE way to grant credit
// — POST /v1/billing/credit-grants (api/billing.CreateCreditGrant, mint-gated) —
// and the "$5 starter" is just that endpoint composed with these values
// (amountCents=StarterCreditCents, tags=StarterCreditTag, expiry=StarterCreditDays).
// Any program (the ai/cloud on-first-use trigger, the admin cockpit, the CLI)
// imports these constants and calls the one grant primitive; there is no bespoke
// grant helper here to duplicate that path.
package credit

// Starter credit constants — the ONE canonical starter/welcome grant values.
//
// The welcome/starter credit is a TRIAL (non-cash) grant: $5.00, classified by
// its `starter-credit` tag. $5 is the ONE canonical amount, shared across the
// stack: billing app config `trialCreditCents: 500`, ai `StarterCreditDollars =
// 5.00`, and the cloud-api-models `features.starter_credit: 5.0`. Keep them equal.
const (
	StarterCreditCents = 500 // $5.00 USD
	StarterCreditDays  = 365 // expires in 365 days
	StarterCreditTag   = "starter-credit"
)
