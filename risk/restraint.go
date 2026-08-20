// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"errors"
	"math"
	"time"

	"github.com/hanzoai/commerce/models/control"
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
	// Reserve names the reserve control that set the share, when one did, so the
	// ledger entry can cite the restraint that required it.
	Reserve string
	// Room is what was still under the reserve's CEILING before this move. It is
	// how a reader tells "withheld nothing because there is no reserve" from
	// "withheld nothing because the reserve is already full".
	Room currency.Cents
}

// Action is what the restraint ALONE says about the move, in the same
// vocabulary the scoring plane speaks — so the two compose with [Strictest]
// rather than with an if, and a control can only ever tighten a judgement.
func (r Restraint) Action() Action {
	if r.Blocked {
		return Block
	}
	return Allow
}

// errRate refuses a reserve rate outside 0..100%. A rate a caller can state
// outside that range is a reserve that would either do nothing or withhold more
// money than the move contains.
var errRate = errors.New("risk: a reserve rate is basis points in 1..10000")

// errCap refuses a reserve with no ceiling. A share of every outbound move with
// no stated total is not a reserve, it is an open-ended seizure: the rate
// applies forever, so the amount withheld is bounded only by how much money the
// merchant tries to move. A reserve states what it may hold, in exact minor
// units, and stops there — enforced against the reserve LEDGER, so the
// accounting is cumulative and not per-move.
var errCap = errors.New("risk: a reserve states a cap in exact minor units above zero")

const rateFloor, rateCeil = 0, control.FullRate

// maxCents is the largest amount [Restrain] will multiply by a rate without
// overflowing int64. Above it the arithmetic would silently wrap, which on a
// money path is the one failure that must never be silent.
const maxCents = math.MaxInt64 / control.FullRate

// Restrain applies the controls in force to one move and reports what may
// happen. held is what this subject's reserve LEDGER already withholds, and it
// is what the ceiling is measured against.
//
// Composition of two controls is the STRICTEST of them, never their sum: two
// 60% reserves are a 60% reserve, because summing them would withhold more than
// the money that exists.
//
// Rounding is UP on the withheld share, so a platform that declares a 25%
// reserve never holds back less than a quarter of a move that does not divide
// evenly. The cent the rounding creates comes out of Allowed, never out of thin
// air — Held+Allowed is the requested amount exactly.
//
// A reserve and a hold bear only on money LEAVING. Money arriving is stopped by
// a block and by nothing else: withholding a share of an inbound charge would
// mean refusing part of a payment, which is not what a reserve is.
func Restrain(controls []*control.Control, amount currency.Cents, out bool, now time.Time, held currency.Cents) Restraint {
	r := Restraint{Allowed: amount}
	if amount < 0 {
		// A negative amount is not a smaller move, it is a move in the other
		// direction wearing the wrong sign. Refuse rather than reason about it.
		return Restraint{Blocked: true, Held: 0, Allowed: 0, Reason: "amount is negative"}
	}

	var rate, ceiling int64
	for _, c := range controls {
		if c == nil || !c.Live(now) {
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
				rate, ceiling = c.Rate, c.Cap
				r.Reserve = c.Ref()
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
		return r
	}

	// The CEILING, measured against what the ledger already holds. A reserve
	// that has taken everything it declared takes nothing more: the money keeps
	// moving and it is the RESERVE that is finished, not the merchant.
	room := currency.Cents(ceiling) - held
	if room < 0 {
		room = 0
	}
	r.Room = room
	if room == 0 {
		return r
	}

	// A full reserve withholds everything, and an amount too large to multiply
	// exactly is treated the same way rather than wrapped — but neither may take
	// more than the ceiling still has room for.
	share := amount
	if rate < rateCeil && amount <= maxCents {
		share = ceilShare(amount, rate)
	}
	if share > room {
		share = room
	}
	r.Held = share
	r.Allowed = amount - share
	return r
}

// ceilShare is rate basis points of amount, rounded up, in integer arithmetic.
// amount is bounded by maxCents so the product cannot overflow.
func ceilShare(amount currency.Cents, rate int64) currency.Cents {
	n := int64(amount)*rate + (control.FullRate - 1)
	return currency.Cents(n / control.FullRate)
}
