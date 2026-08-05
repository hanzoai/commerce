package checkout

import (
	"testing"

	"github.com/hanzoai/commerce/models/coupon"
)

// TestApplyDiscountRounds pins the rounding rule on the checkout path.
//
// This is the same coupon the order model applies, so it must return the same number. It
// used integer division — subtotalCents * pct / 100 — which is exact but floors, so it
// agreed with the order model only because the order model was also wrong. `old` is what
// it returned before.
func TestApplyDiscountRounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		subtotal int64
		pct      int
		want     int64
		old      int64
	}{
		{"19.99 @ 10%", 1999, 10, 200, 199},
		{"19.99 @ 15%", 1999, 15, 300, 299},
		{"19.99 @ 33%", 1999, 33, 660, 659},
		{"9.95 @ 10%", 995, 10, 100, 99},
		{"9.95 @ 15%", 995, 15, 149, 149},
		{"33.33 @ 15%", 3333, 15, 500, 499},
		{"0.29 @ 10%", 29, 10, 3, 2},
		{"0.29 @ 33%", 29, 33, 10, 9},
		{"exact", 1000, 10, 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpn := &coupon.Coupon{Type: coupon.Percent, Amount: tc.pct, Enabled: true}
			if got := applyDiscount(tc.subtotal, cpn); got != tc.want {
				t.Errorf("applyDiscount(%d, %d%%) = %d, want %d (integer truncation gave %d)",
					tc.subtotal, tc.pct, got, tc.want, tc.old)
			}
		})
	}
}

// TestApplyDiscountCaps — rounding must not let a discount exceed the subtotal, and a flat
// coupon larger than the order is still capped.
func TestApplyDiscountCaps(t *testing.T) {
	cpn := &coupon.Coupon{Type: coupon.Percent, Amount: 150, Enabled: true}
	if got := applyDiscount(1999, cpn); got != 1999 {
		t.Errorf("150%% coupon on 1999 = %d, want it capped at 1999", got)
	}
	flat := &coupon.Coupon{Type: coupon.Flat, Amount: 5000, Enabled: true}
	if got := applyDiscount(1999, flat); got != 1999 {
		t.Errorf("$50 flat coupon on 1999 = %d, want it capped at 1999", got)
	}
	if got := applyDiscount(1999, nil); got != 0 {
		t.Errorf("nil coupon = %d, want 0", got)
	}
}
