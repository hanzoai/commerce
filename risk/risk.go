// Copyright © 2026 Hanzo AI. MIT License.

// Package risk is the money plane's seam to Hanzo Risk.
//
// Commerce owns the money — authorizations, payouts, balances, disputes. It does
// not own judgement. `/v1/risk` scores; this package asks it, records the answer
// against the org's own books, and enforces the CONTROLS a platform places on a
// merchant (reserve, payout hold, block). Risk declares, the money plane
// enforces, and the two never trade places: nothing here scores, and nothing at
// /v1/risk moves money.
//
// PROCESSOR-AGNOSTIC BY CONSTRUCTION. [Guard] decorates any
// processor.PaymentProcessor — one gateway, the multi-processor router, or a
// merchant's own — so an authorization is screened wherever it is processed. A
// merchant who does not process payments with us reaches the same judgement
// through POST /v1/billing/risk/screen: it takes the facts and returns the
// decision, touching no processor at all.
//
// FAIL POLICY. Scoring is a network hop and money is not. A scoring plane that
// cannot answer must never take the payment plane down with it, so an
// unreachable plane yields Action=allow with a stated [Decision.Refusal] — the
// refusal is RECORDED, never dropped, because silence must not read as a clean
// result. CONTROLS are the other way round: they are durable rows in the org's
// own store, so they are enforced with no network in the path and a scoring
// outage cannot lift a reserve.
package risk

import (
	"context"
	"errors"
	"sync"

	"github.com/hanzoai/commerce/models/types/currency"
)

// Stage is the lifecycle moment being judged. It selects the feature window and
// the rule set on the scoring plane; it never selects a different tenant.
type Stage string

const (
	// Payment is authorization time: card testing, transaction fraud, bot abuse.
	Payment Stage = "payment"
	// Usage is metered consumption: pay-as-you-go abuse.
	Usage Stage = "usage"
	// Payout is money leaving: payout fraud, mule destinations.
	Payout Stage = "payout"
	// Dispute is a chargeback or its early warning.
	Dispute Stage = "dispute"
	// Merchant is the standing account review a platform runs continuously.
	Merchant Stage = "merchant"
)

// Action is what the money plane must do with the move it asked about. The
// vocabulary is the scoring plane's; commerce enforces it and adds nothing.
type Action string

const (
	Allow     Action = "allow"
	Challenge Action = "challenge"
	Review    Action = "review"
	Restrict  Action = "restrict"
	Block     Action = "block"
)

// Moves reports whether money may move under this action. Challenge and Review
// summon a human but do not stop the move; Restrict and Block do.
//
// It answers by NAMING the actions that permit, not by excluding the two that
// refuse. An action this plane has not learned — a vocabulary the scoring plane
// grew, or an empty field on a malformed record — must stop the money, and a
// rule written as "not block and not restrict" lets exactly those through.
func (a Action) Moves() bool {
	switch a {
	case Allow, Challenge, Review:
		return true
	default:
		return false
	}
}

// severity orders the actions from most permissive to most restrictive. An
// action this plane does not know is treated as the most restrictive thing it
// could mean: a vocabulary the scoring plane grew and commerce has not learned
// yet must fail closed, not fall through to allow.
func (a Action) severity() int {
	switch a {
	case Allow:
		return 0
	case Challenge:
		return 1
	case Review:
		return 2
	case Restrict:
		return 3
	case Block:
		return 4
	default:
		return 4
	}
}

// Strictest is how two judgements about one move compose. The money plane never
// averages advice and never lets a later opinion loosen an earlier one.
func Strictest(a, b Action) Action {
	if b.severity() > a.severity() {
		return b
	}
	return a
}

// Subject is what is being judged, inside one tenant. Kind is the vocabulary the
// money plane can name, and it deliberately CANNOT name a whole org: restraining
// a tenant is the fleet's entitlement plane, not a merchant control, and a
// subject kind that could name the org would let an org's own admin place — and
// release — a platform restraint on itself.
type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Subject kinds this plane accepts.
const (
	KindMerchant    = "merchant"
	KindCustomer    = "customer"
	KindAccount     = "account"
	KindTransaction = "transaction"
	KindPayout      = "payout"
)

var kinds = map[string]bool{
	KindMerchant:    true,
	KindCustomer:    true,
	KindAccount:     true,
	KindTransaction: true,
	KindPayout:      true,
}

// ErrKind rejects a subject the money plane may not judge or restrain.
var ErrKind = errors.New("risk: subject kind is not one this plane names")

// Valid reports the subject usable, or why it is not. An empty id is refused
// because a control with no subject would cover every move in the tenant.
func (s Subject) Valid() error {
	if !kinds[s.Kind] {
		return ErrKind
	}
	if s.ID == "" {
		return errors.New("risk: subject id is empty")
	}
	return nil
}

// Money is an exact amount: minor units of Currency, never a float, in the
// direction it moves. In is money arriving (a charge), Out is money leaving (a
// payout, a refund).
type Money struct {
	Cents    currency.Cents `json:"cents"`
	Currency currency.Type  `json:"currency"`
	Out      bool           `json:"out"`
}

// Client is the ONE door from the money plane to /v1/risk. Nothing else in
// commerce may call the scoring plane: one door means one place where the
// timeout, the identity forwarding and the refusal vocabulary are decided.
type Client interface {
	// Decide scores one event and returns the decision. It never moves money and
	// never writes to commerce's store.
	Decide(ctx context.Context, ask *Ask) (*Decision, error)
	// Label reports how a scored event actually turned out — a dispute, a
	// chargeback, a refund, a failed payout — so the org's own model learns from
	// its own outcomes.
	Label(ctx context.Context, label *Label) error
}

// Ask is one question put to the scoring plane.
//
// The ORG IS NOT A FIELD. It travels as the caller's forwarded identity, minted
// by IAM and asserted by the gateway. A tenant an argument could name is a
// tenant the caller asserted for itself.
type Ask struct {
	Stage   Stage             `json:"stage"`
	Subject Subject           `json:"subject"`
	Amount  *Money            `json:"amount,omitempty"`
	Signals map[string]string `json:"signals,omitempty"`
	// Idem makes a re-ask under the same key return the same decision, so a
	// retried authorization is scored once and charged once.
	Idem string `json:"idem,omitempty"`
}

// Decision is the scoring plane's answer.
type Decision struct {
	ID     string   `json:"id"`
	Action Action   `json:"action"`
	Score  float64  `json:"score"`
	Agency string   `json:"agency,omitempty"`
	Hits   []string `json:"hits,omitempty"`
	Shadow bool     `json:"shadow,omitempty"`
	// Refusal states why the plane could not judge — warming, unusable,
	// unreachable. A decision carrying a refusal is not a clean result, and
	// every caller records it.
	Refusal string `json:"refusal,omitempty"`
}

// Label is an outcome fed back to the scoring plane.
type Label struct {
	Decision string  `json:"decision,omitempty"`
	Subject  Subject `json:"subject"`
	Outcome  string  `json:"outcome"`
	Amount   *Money  `json:"amount,omitempty"`
	Note     string  `json:"note,omitempty"`
}

// Refusal reasons this package mints. The scoring plane mints its own
// (warming, unusable); these name the ways the HOP failed.
const (
	// RefusalAbsent means no scoring plane is configured for this deployment.
	RefusalAbsent = "absent"
	// RefusalUnreachable means the plane did not answer inside the budget.
	RefusalUnreachable = "unreachable"
)

// ErrAbsent is returned by the zero client. It is not an error condition of the
// money move: callers turn it into a recorded refusal and let the move proceed
// under the controls alone.
var ErrAbsent = errors.New("risk: no scoring plane configured")

var (
	mu       sync.RWMutex
	injected Client
)

// Set installs the scoring plane for this process. The cloud binary injects an
// in-process client when /v1/risk is co-resident; a standalone commerce calls
// [At] with the fleet's api host. Passing nil clears it, which makes every
// screen record RefusalAbsent rather than silently scoring 0.
func Set(c Client) {
	mu.Lock()
	injected = c
	mu.Unlock()
}

// Of returns the installed plane, or the absent one — never nil, so no caller
// has to nil-check the thing it is about to ask.
func Of() Client {
	mu.RLock()
	defer mu.RUnlock()
	if injected == nil {
		return absent{}
	}
	return injected
}

// absent is the zero plane: it refuses, in the vocabulary a caller already
// handles, instead of being nil.
type absent struct{}

func (absent) Decide(context.Context, *Ask) (*Decision, error) { return nil, ErrAbsent }
func (absent) Label(context.Context, *Label) error             { return ErrAbsent }
