package organization

// TestMode reports whether this org transacts in TEST mode. It is the SINGLE
// authority for BOTH the payment environment (Square sandbox vs production) AND
// the ledger (trans.Test, balance bucket, pay.Live). Keeping the charge
// environment and the ledger on ONE authority is what prevents a sandbox charge
// from crediting the live (spendable) balance and a production charge from
// booking test (unbilled) revenue.
//
// The authority is the ORG RECORD, and nothing else. It used to be the
// deployment's SQUARE_ENVIRONMENT, which consulted o.Live only when that
// variable was UNSET — so on any templated deploy every tenant was forced into
// one mode and each org's own flag was dead. That is what made a second
// deployment necessary to serve a sandbox merchant (commerce-api.testnet.hanzo.ai
// exists for exactly this reason), and why a replica was never freely
// interchangeable: its behaviour depended on which env block started it.
// Resolving per org is what lets ONE stateless replica serve a sandbox merchant
// and a live merchant in the same process, on the same request path.
//
// It is also what "configured per org, not in env files" means here. The
// credentials were already per org — o.Square.Sandbox / o.Square.Production,
// o.Stripe.Test / o.Stripe.Live, KMS-backed — and StripeToken() already chose
// between them from o.Live alone. Only the mode stayed deployment-wide, so the
// two could disagree. Now both read the same per-org fact.
//
// Still fail-CLOSED, but per tenant instead of per deployment: an org is in
// production only when its own record says Live, so a new, unset or
// half-configured org transacts in sandbox and a missing flag can never silently
// charge real cards. What improves over the env gate is isolation — one org's
// misconfiguration can no longer drag every other tenant on the pod into the
// wrong environment, in either direction.
func (o Organization) TestMode() bool {
	return !o.Live
}

// SquareEnvironment returns "sandbox" or "production" — the value the Square SDK
// uses to select its API base URL — derived from TestMode (one authority).
func (o Organization) SquareEnvironment() string {
	if o.TestMode() {
		return "sandbox"
	}
	return "production"
}
