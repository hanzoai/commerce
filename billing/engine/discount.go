// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package engine

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/types"
)

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

// Coupon is a percent off the plan fee of one invoice, of one of two kinds.
//
// A plan coupon is a code the subscription was bought with (api/promo). It
// discounts the first Months billing cycles the subscription pays for, counted
// from its first paid period, and Month is which of them the invoice bills. The
// row carries it from the sale (Grant), and an invoice is handed it while fewer
// than Months cycles have been billed before it (couponOf): the first one at the
// sale, the rest by renewals.
//
// A Renewal coupon is the one staff put on a subscription for its next renewal
// (subscription.NextCouponCode). RenewSubscription hands it to the invoice that
// renewal builds and the row stops carrying it.
//
// They are two kinds because they are two promises. A plan coupon was sold for
// the first cycles of a purchase; a renewal coupon is for whichever period the
// next renewal bills, on a month or a year. One row can hold both at once — a
// win-back inside the cycles a sale promised — and they stack, the renewal
// coupon off what the plan coupon left.
type Coupon struct {
	Code    string
	Percent int
	// Months is how many monthly cycles a plan coupon discounts, 1 for a
	// first-month coupon, and Month which of them the invoice bills.
	Month, Months int
	// Renewal marks the staff coupon for the next renewal.
	Renewal bool
}

// Label is how the coupon reads on the invoice it discounts.
func (c Coupon) Label() string {
	if c.Code == "" || c.Percent <= 0 {
		return ""
	}
	off := c.Code + " — " + strconv.Itoa(c.Percent) + "% off"
	switch {
	case c.Renewal:
		return off + " this renewal"
	case c.Months > 1:
		return off + ", month " + strconv.Itoa(max(c.Month, 1)) + " of " + strconv.Itoa(c.Months)
	}
	return off + " first month"
}

// Grant puts plan coupon c on sub, the subscription bought with it, for the plan
// slugs plans: c.Percent off the plan fee of each of its first c.Months billing
// cycles (at least one), counted from from — at a sale the period the row holds
// (sub.PeriodStart), which on a trial is the first paid period after it.
func Grant(sub *subscription.Subscription, c Coupon, plans []string, from time.Time) {
	if c.Code == "" || c.Percent < 1 {
		return
	}
	sub.CouponCode, sub.CouponPercent, sub.CouponMonths = c.Code, c.Percent, max(c.Months, 1)
	sub.CouponPlans = slices.Clone(plans)
	sub.CouponFrom = from
}

// couponOf is the plan coupon sub was bought with as it discounts the period
// starting at start, when billed cycles have been invoiced since its first paid
// period (Billed), or the zero Coupon when it does not. It does while fewer than
// its Months cycles were billed before this one, the row bills monthly, and the
// coupon names the row's plan and every seat type the period bills at the price
// it was sold at. A cycle is an invoice, so days a gift adds, or a late payment
// that moves the period, spend no month; a cycle billed on a plan the coupon
// does not name, or on a year, spends one, and is billed at its price.
func couponOf(sub *subscription.Subscription, start time.Time, billed int) Coupon {
	if sub.CouponCode == "" || sub.CouponPercent < 1 || sub.CouponMonths < 1 {
		return Coupon{}
	}
	if start.Unix() < sub.CouponFrom.Unix() || billed >= sub.CouponMonths || !BillsMonthly(&sub.Plan) {
		return Coupon{}
	}
	if !slices.Contains(sub.CouponPlans, cmp.Or(sub.Plan.Slug, sub.PlanId)) {
		return Coupon{}
	}
	for _, l := range sub.Lines {
		if !l.Full && l.Quantity > 0 && !slices.Contains(sub.CouponPlans, l.Plan) {
			return Coupon{}
		}
	}
	return Coupon{Code: sub.CouponCode, Percent: sub.CouponPercent, Month: billed + 1, Months: sub.CouponMonths}
}

// BillsMonthly reports whether p bills one month at a time, as Advance reads it: the
// only period a plan coupon discounts.
func BillsMonthly(p *plan.Plan) bool {
	return (p.Interval == "" || p.Interval == types.Monthly) && p.IntervalCount <= 1
}
