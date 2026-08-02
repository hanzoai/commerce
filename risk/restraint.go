// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"errors"
	"math"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/types/currency"
)

// Restraint is what the standing controls do to one move. It is a pure value
// computed by [Restrain] from the controls in force — no store, no clock of its
// own, no network — which is why the money plane's hardest rule is also its
// most testable one.
type Restraint struct {
	// Blocked reports that the move must not happen at all.
	Blocked bool
	// Held is the exact minor units withheld from an outbound move, and Allowed
	// what may still leave. Held+Allowed always equals the requested amount, so
	// no cent is created or lost by the arithmetic.
	Held    currency.Cents
	Allowed currency.Cents
	// Controls are the ids that bore on the move, for the record and the appeal.
	Controls []string
	// Reason is the strictest applying control's reason, or empty when nothing
	// applied.
	Reason string
	// Reserve is the control whose rate took Held, or nil when no reserve bore
	// on the move. It is carried because the money that was withheld has to be
	// accounted for AGAINST THE DECLARATION THAT TOOK IT — the ceiling, the
	// running total and the release all live on that control.
	Reserve *control.Control
}

// errRate refuses a reserve rate outside 0..100%. A rate a caller can state
// outside that range is a reserve that would either do nothing or withhold more
// money than the move contains.
var errRate = errors.New("risk: a reserve rate is basis points in 1..10000")

// errCap refuses a ceiling that cannot be reconciled: a negative amount, or an
// amount with no currency to be denominated in.
var errCap = errors.New("risk: a reserve ceiling is a positive amount in a named currency")

const rateFloor, rateCeil = 0, control.FullRate

// maxCents is the largest amount [Restrain] will multiply by a rate without
// overflowing int64. Above it the arithmetic would silently wrap, which on a
// money path is the one failure that must never be silent.
const maxCents = math.MaxInt64 / control.FullRate

// Restrain applies the controls in force to one move and reports what may
// happen. Composition of two controls is the STRICTEST of them, never their
// sum: two 60% reserves are a 60% reserve, because summing them would withhold
// more than the money that exists.
//
// Rounding is UP on the withheld share, so a platform that declares a 25%
// reserve never holds back less than a quarter of a move that does not divide
// evenly. The cent the rounding creates comes out of Allowed, never out of thin
// air — Held+Allowed is the requested amount exactly.
//
// A reserve and a hold bear only on money LEAVING. Money arriving is stopped by
// a block and by nothing else: withholding a share of an inbound charge would
// mean refusing part of a payment, which is not what a reserve is.
//
// A reserve may also name a CURRENCY, and then it bears only on moves in it —
// because a reserve that names a ceiling names an amount, and an amount in one
// currency cannot bound holds taken in another. A hold and a block never narrow
// this way: they are about the subject, not about an amount.
func Restrain(controls []*control.Control, amount currency.Cents, cur currency.Type, out bool, now time.Time) Restraint {
	r := Restraint{Allowed: amount}
	if amount < 0 {
		// A negative amount is not a smaller move, it is a move in the other
		// direction wearing the wrong sign. Refuse rather than reason about it.
		return Restraint{Blocked: true, Held: 0, Allowed: 0, Reason: "amount is negative"}
	}

	var rate int64
	for _, c := range controls {
		if c == nil || !c.Live(now) || !c.Bears(cur) {
			continue
		}
		switch c.Effect {
		case control.Block:
			r.Blocked = true
			r.Controls = append(r.Controls, c.Ref())
			r.Reason = c.Reason
		case control.Hold:
			if out {
				r.Blocked = true
				r.Controls = append(r.Controls, c.Ref())
				if r.Reason == "" {
					r.Reason = c.Reason
				}
			}
		case control.Reserve:
			if out && c.Rate > rate {
				rate = c.Rate
				r.Reserve = c
				r.Controls = append(r.Controls, c.Ref())
				if r.Reason == "" {
					r.Reason = c.Reason
				}
			}
		}
	}

	if r.Blocked {
		return Restraint{Blocked: true, Held: amount, Allowed: 0, Controls: r.Controls, Reason: r.Reason}
	}
	if rate <= rateFloor {
		r.Reserve = nil
		return r
	}
	if rate >= rateCeil || amount > maxCents {
		// A full reserve withholds everything, and an amount too large to
		// multiply exactly is treated the same way rather than wrapped.
		return Restraint{Held: amount, Allowed: 0, Controls: r.Controls, Reason: r.Reason, Reserve: r.Reserve}
	}

	r.Held = ceilShare(amount, rate)
	r.Allowed = amount - r.Held
	return r
}

// Cap clamps what a restraint withholds to the headroom left under the reserve
// that took it, and hands the cents it releases back to Allowed — so Held plus
// Allowed is still the requested amount exactly.
//
// It is separate from [Restrain] and pure for the same reason [Restrain] is:
// what the controls SAY (a rate) and what the account ALLOWS (a ceiling and a
// running total) are two different questions, and braiding them would make
// neither testable. A reserve with no ceiling is unchanged by this, so the
// composition is safe to apply always and there is no branch to forget.
//
// The clamp itself is [reserve.Grant] and is not restated here: the judgement
// applies it against [reserve.Headroom], a read that forecasts what a payout
// would withhold, and the disbursement applies THE SAME FUNCTION against the
// total the store holds inside its own transaction, which is the one that
// decides. Two headrooms, one arithmetic — spelled twice they would be two
// arithmetics, and a forecast that disagrees with the decision by a cent is a
// payout a merchant cannot reconcile.
//
// A ceiling never loosens a BLOCK: a blocked move withholds everything by not
// happening, which is not a reserve taking money and not something a ceiling
// has any claim on.
func Cap(r Restraint, headroom currency.Cents) Restraint {
	if r.Blocked || r.Held <= 0 {
		return r
	}
	granted := currency.Cents(reserve.Grant(int64(r.Held), int64(headroom)))
	if granted == r.Held {
		return r
	}
	amount := r.Held + r.Allowed
	r.Held = granted
	r.Allowed = amount - granted
	if r.Held == 0 {
		// The ceiling is reached: this reserve withholds nothing further, so it
		// is not the control that bore on this move and must not be recorded as
		// having taken anything from it.
		r.Reserve = nil
	}
	return r
}

// ceilShare is rate basis points of amount, rounded up, in integer arithmetic.
// amount is bounded by maxCents so the product cannot overflow.
func ceilShare(amount currency.Cents, rate int64) currency.Cents {
	n := int64(amount)*rate + (control.FullRate - 1)
	return currency.Cents(n / control.FullRate)
}
