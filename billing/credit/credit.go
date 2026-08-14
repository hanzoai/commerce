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
	StarterCreditTag   = "starter-credit"
)

// LifetimeDays is how long ANY credit is good for.
//
// It was the starter grant's own number, which made "credit expires" a property
// of one promotion rather than of credit. A deposit took an expiry only when the
// caller passed `expiresIn`, and no caller did — so money granted with a deadline
// sat next to money bought without one, and the ledger carried two kinds of
// credit that looked identical in every balance it served.
//
// One year, one value, every credit. The mechanism was already here and enforced
// — transaction.ExpiresAt is excluded from balance, and creditgrant consumes by
// priority then earliest expiry — so this only decides the date those put on a
// credit that arrives without one.
const LifetimeDays = 365

// StarterCreditDays is the starter grant's expiry, which is simply the lifetime
// every other credit now gets. Kept as its own name because the grant's callers
// read it, and pointed at the one value so the two cannot drift apart.
const StarterCreditDays = LifetimeDays
