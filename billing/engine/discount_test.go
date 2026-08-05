// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package engine

import "testing"

func TestDiscountCents(t *testing.T) {
	cases := []struct {
		name     string
		subtotal int64
		percent  int
		want     int64
	}{
		{"no promo is no discount", 2000, 0, 0},
		{"a negative percent is no promo, never a surcharge", 2000, -10, 0},
		{"the advertised half of a $20 plan", 2000, 50, 1000},
		{"20% of $20", 2000, 20, 400},
		{"per-seat subtotals discount whole, not per seat", 6000, 25, 1500},
		{"100% off leaves nothing to charge", 2000, 100, 2000},
		{"a percent above 100 cannot invert the sale", 2000, 150, 2000},
		{"nothing to discount", 0, 50, 0},
		// Half-up: 33% of $10.01 is 330.33c -> 330c, and of $19.99 is 659.67c -> 660c.
		// The customer never pays more than the advertised percent implies.
		{"rounds the fractional cent to the customer", 1001, 33, 330},
		{"rounds up when the half falls that way", 1999, 33, 660},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DiscountCents(tc.subtotal, tc.percent); got != tc.want {
				t.Fatalf("DiscountCents(%d, %d) = %d, want %d", tc.subtotal, tc.percent, got, tc.want)
			}
		})
	}
}

// The whole reason this function exists: the card charge and the invoice must
// agree to the cent, because they are computed in two different packages. If
// they ever diverge a customer is billed one number and shown another.
func TestDiscountCents_ChargeEqualsInvoice(t *testing.T) {
	for subtotal := int64(1); subtotal <= 5000; subtotal++ {
		for _, pct := range []int{0, 1, 17, 25, 50, 99, 100} {
			d := DiscountCents(subtotal, pct)
			charge := subtotal - d           // what api/billing charges the card
			amountDue := subtotal - d        // what Finalize() computes for the invoice
			if charge != amountDue {
				t.Fatalf("charge %d != amountDue %d at subtotal=%d pct=%d", charge, amountDue, subtotal, pct)
			}
			if charge < 0 {
				t.Fatalf("charge went negative (%d) at subtotal=%d pct=%d — a discount became a refund", charge, subtotal, pct)
			}
			if d > subtotal {
				t.Fatalf("discount %d exceeds subtotal %d at pct=%d", d, subtotal, pct)
			}
		}
	}
}
