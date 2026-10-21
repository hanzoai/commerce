// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package engine

import "strconv"

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

// Discounted is fee less each percent in turn, every percent priced off what the
// one before it left: the subscription's promo, then a coupon. The card path and
// the invoice both call it with the same percents in the same order, so the
// amount charged equals the invoice's AmountDue however many discounts stack.
func Discounted(fee int64, percents ...int) int64 {
	for _, p := range percents {
		fee -= DiscountCents(fee, p)
	}
	return fee
}

// Coupon is a percent off ONE invoice. A plan coupon code discounts the first
// period of the subscription bought with it: it is handed to the invoice that
// period's charge paid and never stored on the subscription. A Renewal coupon is
// the one staff put on a subscription for its next renewal
// (subscription.NextCouponCode): RenewSubscription hands it to the invoice that
// renewal builds and the row stops carrying it. Every other period bills at the
// plan's price, less only the promo the subscription carries.
type Coupon struct {
	Code    string
	Percent int
	// Renewal marks a coupon a renewal invoice carries, which reads as off that
	// renewal rather than off the first month.
	Renewal bool
}

// Label is how the coupon reads on the invoice it discounts.
func (c Coupon) Label() string {
	if c.Code == "" || c.Percent <= 0 {
		return ""
	}
	if c.Renewal {
		return c.Code + " — " + strconv.Itoa(c.Percent) + "% off this renewal"
	}
	return c.Code + " — " + strconv.Itoa(c.Percent) + "% off first month"
}
