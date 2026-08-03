package solanapay

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

// The Solana Pay `amount` parameter is the token's own scale, not a fiat one:
// USDC/USDT are 6-decimal SPL tokens and native SOL is 9-decimal lamports.
// These pin that scale, and that the rendering is fixed-scale — the hand-rolled
// version trimmed a whole amount to "1" and mis-signed anything negative.
func TestFormatAmount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cur    currency.Type
		amount currency.Cents
		want   string
	}{
		{"usdc whole", "usdc", 1_000_000, "1.000000"},
		{"usdc fractional", "usdc", 1_500_000, "1.500000"},
		{"usdc one minor unit", "usdc", 1, "0.000001"},
		{"usdc zero", "usdc", 0, "0.000000"},
		{"usdt whole", "usdt", 2_000_000, "2.000000"},
		{"sol whole", "sol", 1_000_000_000, "1.000000000"},
		{"sol fractional", "sol", 1_500_000_000, "1.500000000"},
		{"sol one lamport", "sol", 1, "0.000000001"},
		// A refund is negative, and the sign belongs in front of the whole
		// amount. Printing amount/scale, a dot, then amount%scale put it in the
		// middle, because Go's % keeps the sign of the dividend.
		{"usdc refund", "usdc", -1_500_000, "-1.500000"},
		{"sol refund", "sol", -1_500_000_000, "-1.500000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatAmount(tc.cur, tc.amount); got != tc.want {
				t.Errorf("formatAmount(%s, %d) = %q, want %q", tc.cur, tc.amount, got, tc.want)
			}
		})
	}
}

// An unrecognized currency is native SOL, matching mintForCurrency's default.
func TestTokenCurrencyDefaultsToSOL(t *testing.T) {
	for _, c := range []currency.Type{"sol", "", "unknown"} {
		if got := tokenCurrency(c).Decimals; got != 9 {
			t.Errorf("tokenCurrency(%q).Decimals = %d, want 9", c, got)
		}
	}
	for _, c := range []currency.Type{"usdc", "usdt"} {
		if got := tokenCurrency(c).Decimals; got != 6 {
			t.Errorf("tokenCurrency(%q).Decimals = %d, want 6", c, got)
		}
	}
}
