package currency

import (
	"github.com/hanzoai/decimal"
	"github.com/hanzoai/money"
)

// Scale returns c scaled by an exact rate — a discount, a commission, a tax — rounded to a
// whole minor unit HALF AWAY FROM ZERO. It is the one way to take a percentage of an
// amount in this codebase; the arithmetic itself is money.ScaleMinor, for the same reason
// the parser above is money.ParseCents.
//
// The rule is rounding, not truncation, and it is the rule everywhere rather than one rule
// for discounts and another for fees. 10% off $19.99 is $1.999: truncating gives $1.99 and
// keeps the tenth of a cent, rounding gives $2.00 and gives it away. Either is defensible
// once; what is not defensible is that the fraction always fell the same way, so a price
// book's worth of orders each quietly withheld a fraction of a cent from the customer and
// the sum was never zero.
//
// Half away from zero also means a negative scales like the positive it reverses, so a
// refunded discount is the discount back — which math.Floor, rounding toward negative
// infinity, does not give.
//
// PANICS if the result does not fit an int64. Cents is an int64, so the only inputs that
// can do this are an amount already near the type's limit scaled up by a rate above 1 — no
// order reaches $46 quadrillion, and a discount rate is at most 1 anyway. Returning a
// wrapped number instead would report money nobody owes, and returning zero would report a
// free order; this follows money.Amount.Cmp, which panics across currencies on the same
// reasoning: it is a bug in the caller, not a runtime condition to branch on.
func (c Cents) Scale(rate decimal.Decimal) Cents {
	scaled, err := money.ScaleMinor(int64(c), rate)
	if err != nil {
		panic("currency: " + err.Error())
	}
	return Cents(scaled)
}

// Percent returns pct percent of c, rounded half away from zero — the shape coupons and
// discount rules store a percentage in (10 means 10%, not 0.10).
func (c Cents) Percent(pct int) Cents {
	return c.Scale(decimal.New(int64(pct), 2))
}

// BasisPoints returns bps basis points of c, rounded half away from zero — the shape
// promotion application methods store a percentage in (1500 means 15.00%).
func (c Cents) BasisPoints(bps int) Cents {
	return c.Scale(decimal.New(int64(bps), 4))
}
