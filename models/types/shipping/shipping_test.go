package shipping

import (
	"math"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/weight"
)

// TestVariableShippingRoundsUpTheAmountNotTheFloat pins the direction AND the exactness of
// a variable shipping rate. Rounding up is deliberate and stays: a variable rate bills the
// weight, and a part-cent of carriage is charged, not absorbed.
//
// What is not deliberate is rounding up an amount that had nothing to round. A 0.07 kg item
// at 700 cents/kg is exactly 49 cents, but the float holds 0.07 a hair high, so the product
// is 49.000000000000007 and math.Ceil bills 50. The customer pays a cent for a fraction that
// does not exist.
func TestVariableShippingRoundsUpTheAmountNotTheFloat(t *testing.T) {
	for _, tc := range []struct {
		w     weight.Mass
		price currency.Cents
		want  currency.Cents
		why   string
	}{
		// Exact products: rounding up must be a no-op.
		{0.07, 700, 49, "0.07kg × 700c/kg is exactly 49c"},
		{0.7, 700, 490, "0.7kg × 700c/kg is exactly 490c"},
		{0.29, 100, 29, "0.29kg × 100c/kg is exactly 29c"},
		{2, 1250, 2500, "whole weight, whole price"},
		{0, 700, 0, "no weight, no carriage"},
		// Genuine fractions: still rounded up, that is the policy.
		{0.155, 700, 109, "108.5 rounds UP to 109, not to 108"},
		{1.5, 999, 1499, "1498.5 rounds UP"},
	} {
		if got := calculateShippingPrice(tc.w, Variable, tc.price); got != tc.want {
			t.Errorf("calculateShippingPrice(%v, Variable, %d) = %d, want %d — %s",
				tc.w, tc.price, got, tc.want, tc.why)
		}
	}
}

// TestFlatShippingIgnoresWeight — the non-variable branch is unchanged.
func TestFlatShippingIgnoresWeight(t *testing.T) {
	if got := calculateShippingPrice(3.7, Flat, 500); got != 500 {
		t.Errorf("flat rate = %d, want 500", got)
	}
}

// TestVariableShippingSurvivesAnUnpricableWeight: weight.Convert divides by the target
// unit's gram factor, so a rate row with an unset WeightUnit yields +Inf, and a zero-weight
// product through that same rate yields NaN. Neither can price a variable rate. The old
// arithmetic converted them straight to Cents, which in Go is undefined for a non-finite
// float: measured here it billed 9223372036854775807 for +Inf, -9223372036854775808 for
// -Inf and 0 for NaN. The row's own configured price is the defined answer.
func TestVariableShippingSurvivesAnUnpricableWeight(t *testing.T) {
	inf := weight.Mass(math.Inf(1))
	nan := weight.Mass(math.NaN())
	for _, w := range []weight.Mass{inf, -inf, nan} {
		got := calculateShippingPrice(w, Variable, 500)
		if got != 500 {
			t.Errorf("calculateShippingPrice(%v, Variable, 500) = %d, want the configured 500", w, got)
		}
	}
}
