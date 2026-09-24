package currency

import (
	"math"
	"testing"

	"github.com/hanzoai/decimal"
	"github.com/hanzoai/money"
)

// TestPercentRounds pins the rule at the one place every discount in the codebase now goes
// through. `old` is what the truncating spellings this replaced returned.
func TestPercentRounds(t *testing.T) {
	for _, tc := range []struct {
		cents Cents
		pct   int
		want  Cents
		old   Cents
	}{
		{1999, 10, 200, 199},
		{1999, 15, 300, 299},
		{1999, 33, 660, 659},
		{995, 10, 100, 99},
		{995, 15, 149, 149},
		{995, 33, 328, 328},
		{3333, 10, 333, 333},
		{3333, 15, 500, 499},
		{3333, 33, 1100, 1099},
		{29, 10, 3, 2},
		{29, 15, 4, 4},
		{29, 33, 10, 9},
		{0, 33, 0, 0},
		{1000, 10, 100, 100},
		{1999, 100, 1999, 1999},
		{1999, 0, 0, 0},
	} {
		if got := tc.cents.Percent(tc.pct); got != tc.want {
			t.Errorf("Cents(%d).Percent(%d) = %d, want %d (truncation gave %d)",
				tc.cents, tc.pct, got, tc.want, tc.old)
		}
	}
}

// TestPercentIsSymmetricAroundZero — a refund of a discount is the discount, so scaling a
// negative rounds away from zero too. math.Floor, which this replaced, rounds toward
// negative infinity and would give -200 where truncation gave -199; neither is a mirror of
// the positive case, and half away from zero is.
func TestPercentIsSymmetricAroundZero(t *testing.T) {
	for _, cents := range []Cents{1999, 995, 3333, 29, 1, 12345} {
		for _, pct := range []int{10, 15, 33, 7} {
			pos, neg := cents.Percent(pct), (-cents).Percent(pct)
			if pos != -neg {
				t.Errorf("Cents(%d).Percent(%d) = %d but the negative gave %d; want mirrored",
					cents, pct, pos, neg)
			}
		}
	}
}

// TestBasisPoints — the promotion spelling. 1500 bps is 15%.
func TestBasisPoints(t *testing.T) {
	for _, tc := range []struct {
		cents Cents
		bps   int
		want  Cents
	}{
		{1999, 1500, 300}, // 15% — 299.85
		{1999, 1000, 200},
		{1999, 3300, 660},
		{29, 1000, 3},
		{10000, 1, 1},     // 0.01% of $100.00 is exactly 1 cent
		{10000, 250, 250}, // 2.5% of $100.00 is $2.50
	} {
		if got := tc.cents.BasisPoints(tc.bps); got != tc.want {
			t.Errorf("Cents(%d).BasisPoints(%d) = %d, want %d", tc.cents, tc.bps, got, tc.want)
		}
	}
	// Basis points and percent must agree where they express the same rate.
	for _, cents := range []Cents{1999, 995, 3333, 29, 1, 7, 88888} {
		for _, pct := range []int{1, 7, 10, 15, 33, 50, 99} {
			if p, b := cents.Percent(pct), cents.BasisPoints(pct*100); p != b {
				t.Errorf("Cents(%d): Percent(%d)=%d but BasisPoints(%d)=%d", cents, pct, p, pct*100, b)
			}
		}
	}
}

// TestScaleIsExactPastFloat64 — the rate is applied in big.Int, so an amount past the
// float64 integer limit scales exactly. The old spelling could not represent these.
func TestScaleIsExactPastFloat64(t *testing.T) {
	const big = Cents(1) << 60 // far past 2^53
	if got, want := big.Percent(50), big/2; got != want {
		t.Errorf("Cents(2^60).Percent(50) = %d, want %d", got, want)
	}
	if got, want := big.Scale(decimal.New(1, 0)), big; got != want {
		t.Errorf("scaling by 1 changed the value: %d != %d", got, want)
	}
}

// TestScaleOverflowPanics — a scaled amount that does not fit is a bug in the caller, and
// it must not become a wrapped number or a silent zero.
func TestScaleOverflowPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("scaling MaxInt64 cents by 2 returned a value; want a panic")
		}
	}()
	_ = Cents(math.MaxInt64).Scale(decimal.New(2, 0))
}

// TestScaleCeilAndFloorRoundTheAmountNotTheFloat pins the two directed scalings at the one
// place every fee in the codebase now goes through. `oldCeil`/`oldFloor` are what the
// math.Ceil/math.Floor spellings they replaced returned.
//
// Every row here has an EXACT product — a whole number of cents, nothing to round. That is
// the point: rounding a whole number is a no-op in either direction, and the float
// spellings still moved it, because they rounded the float rather than the amount.
func TestScaleCeilAndFloorRoundTheAmountNotTheFloat(t *testing.T) {
	for _, tc := range []struct {
		cents             Cents
		pct               float64
		want              Cents
		oldCeil, oldFloor Cents
	}{
		{700, 0.07, 49, 50, 49}, // 0.07 is held a hair HIGH: Ceil invented a cent
		{7000, 0.07, 490, 491, 490},
		{100, 0.29, 29, 29, 28}, // 0.29 is held a hair LOW: Floor destroyed one
		{2900, 0.01, 29, 29, 29},
		{300, 0.10, 30, 30, 30},
		{8300, 0.03, 249, 249, 249},
		{100000, 0.029, 2900, 2900, 2900},
	} {
		rate, err := money.RateFromFloat(tc.pct)
		if err != nil {
			t.Fatalf("RateFromFloat(%v): %v", tc.pct, err)
		}
		if got := tc.cents.ScaleCeil(rate); got != tc.want {
			t.Errorf("Cents(%d).ScaleCeil(%v) = %d, want %d (math.Ceil gave %d)",
				tc.cents, tc.pct, got, tc.want, tc.oldCeil)
		}
		if got := tc.cents.ScaleFloor(rate); got != tc.want {
			t.Errorf("Cents(%d).ScaleFloor(%v) = %d, want %d (math.Floor gave %d)",
				tc.cents, tc.pct, got, tc.want, tc.oldFloor)
		}
	}
}

// TestScaleCeilAndFloorKeepTheirDirection: on a product that DOES have a fraction, each
// still rounds the way its callers depend on — up for a fee we collect, down for a
// commission we pay out and keep the remainder of. Negatives go toward the named infinity,
// not away from zero, so a reversed fee does not gain a cent on the way back.
func TestScaleCeilAndFloorKeepTheirDirection(t *testing.T) {
	for _, tc := range []struct {
		cents               Cents
		pct                 int
		wantCeil, wantFloor Cents
	}{
		{1999, 10, 200, 199},    // 199.9
		{995, 10, 100, 99},      // 99.5
		{29, 33, 10, 9},         // 9.57
		{1250, 6, 75, 75},       // 75 exactly
		{-1999, 10, -199, -200}, // -199.9
	} {
		rate := decimal.New(int64(tc.pct), 2)
		if got := tc.cents.ScaleCeil(rate); got != tc.wantCeil {
			t.Errorf("Cents(%d).ScaleCeil(%d%%) = %d, want %d", tc.cents, tc.pct, got, tc.wantCeil)
		}
		if got := tc.cents.ScaleFloor(rate); got != tc.wantFloor {
			t.Errorf("Cents(%d).ScaleFloor(%d%%) = %d, want %d", tc.cents, tc.pct, got, tc.wantFloor)
		}
	}
}
