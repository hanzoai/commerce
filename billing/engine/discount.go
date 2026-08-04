// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package engine

// DiscountCents is the ONE place a percent-off becomes money.
//
// It exists because the charge and the invoice are computed in two different
// packages — the card path in api/billing prices the first period, the engine
// prices every period after it — and the invariant they must satisfy is that
// the amount charged EQUALS the invoice's AmountDue. Two copies of "subtract a
// percent" is how a customer gets billed one number and shown another; one
// function is how they cannot.
//
// Rounding is half-up on the discount, so a fractional cent lands in the
// customer's favour: they are never charged more than the advertised percent
// implies. A percent outside 1..100 discounts nothing (0 and negatives are "no
// promo"; >100 would invert the sale), and the result never exceeds the
// subtotal, so a discount cannot turn a charge into a refund.
func DiscountCents(subtotalCents int64, percent int) int64 {
	if subtotalCents <= 0 || percent <= 0 {
		return 0
	}
	if percent >= 100 {
		return subtotalCents
	}
	d := (subtotalCents*int64(percent) + 50) / 100
	if d > subtotalCents {
		return subtotalCents
	}
	return d
}
