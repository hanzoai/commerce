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
	return o.kind().Word("square")
}

// A Kind is what an environment transacts with. There are two of them, and
// every payment processor has the same two under a different name — which is
// the whole reason to state them once here. Stripe says live and test, Square
// says production and sandbox, Braintree says production and sandbox, Adyen
// says live and test. Spelling a processor's word at each of the places that
// talks to it is how a sandbox key comes to be sent to a live endpoint.
type Kind string

const (
	Live Kind = "live"
	Test Kind = "test"
)

// An org may hold as many environments as it likes, and each one CHOOSES its
// kind. That is the point of choosing rather than deriving it from the name: a
// team can run an experimental build of its platform against real money while a
// stable one keeps serving customers, and can keep several test environments
// beside both. Deriving the kind from the name would have allowed exactly one
// live environment, which is the arrangement this replaces.

// word is what each processor calls the two kinds, live first. This table is
// the only place in commerce that knows a processor's spelling; everything else
// asks for the Kind and lets the processor's own edge translate it.
var word = map[string][2]string{
	"stripe":       {"live", "test"},
	"square":       {"production", "sandbox"},
	"authorizenet": {"production", "sandbox"},
	"paypal":       {"live", "sandbox"},
	"adyen":        {"live", "test"},
	"braintree":    {"production", "sandbox"},
	"lemonsqueezy": {"live", "test"},
	"mercury":      {"production", "sandbox"},
}

// Word is how one processor spells this kind. A processor nobody has taught us
// about gets our own word, which is wrong in a way that is visible at its edge
// rather than wrong in a way that charges the wrong merchant.
func (k Kind) Word(processor string) string {
	w, ok := word[processor]
	if !ok {
		return string(k)
	}
	if k == Live {
		return w[0]
	}
	return w[1]
}

// kind reports what this org transacts with as it stands. Unexported because
// the ORM's entity interface already claims the name Kind for the storage kind,
// and two different answers to "what kind is this" is how the wrong one gets
// read.
func (o Organization) kind() Kind {
	if o.Live {
		return Live
	}
	return Test
}

// In returns this org as it transacts in an environment of one kind.
//
// The environment decides live-vs-test, and this is where it decides it — once,
// on the value, rather than at each of the sixty places that ask. Live already
// selects the credential pair, the ledger's books and pay.Live, so setting it
// here carries the environment to all of them without any of them learning a
// new word. The receiver is a value: the org this returns is a copy scoped to
// one request, and the stored record is never touched.
//
// Anything that is not exactly Live is Test, so an environment that arrives
// with no kind, or with a kind nobody recognises, charges test credentials.
func (o Organization) In(k Kind) Organization {
	o.Live = k == Live
	return o
}
