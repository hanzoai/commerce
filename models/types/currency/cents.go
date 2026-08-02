package currency

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type Cents int64

// CentsFromString converts a decimal amount of major units ("19.99") into
// minor units (1999).
//
// It parses exactly, never through float64. The previous implementation was
// `int64(f * 100)` on a parsed float, which loses a cent on ordinary prices:
// 19.99 has no exact binary representation, so the product is 1998.999… and
// truncating gives 1998. Measured against real price points — 19.99→1998,
// 9.95→994, 1.15→114, 0.29→28 — every one undercharging by a cent on money
// that had already been captured.
//
// It also returns the parse error instead of discarding it. Mapping an
// unparseable amount to zero is a free transaction that reports success.
func CentsFromString(s string) (Cents, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("currency: %q is not a decimal amount: %w", s, err)
	}
	// Shift by two and round half away from zero: an amount already at cent
	// precision — which every captured payment is — converts exactly, and a
	// negative (a refund) never rounds toward zero.
	return Cents(d.Shift(2).Round(0).IntPart()), nil
}
