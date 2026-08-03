package circle

import (
	"errors"
	"testing"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
)

// Circle settles in USD/USDC at two decimals, and this file converts in both
// directions: currency.Type renders what we send, parseAmount reads what comes
// back. Both spellings used to be hand-rolled, and both were wrong.

// The outbound amount used to be printed as cents/100, a dot, then cents%100.
// Go's % keeps the sign of the dividend, so a refund rendered "-19.-99".
func TestOutboundAmountRendersNegatives(t *testing.T) {
	for _, tc := range []struct {
		cents currency.Cents
		want  string
	}{
		{1050, "10.50"},
		{1099, "10.99"},
		{0, "0.00"},
		{1, "0.01"},
		{-1999, "-19.99"},
		{-1, "-0.01"},
	} {
		if got := currency.USD.ToStringNoSymbol(tc.cents); got != tc.want {
			t.Errorf("USD.ToStringNoSymbol(%d) = %q, want %q", tc.cents, got, tc.want)
		}
	}
}

// parseAmount used to scan "%d.%d" and compute whole*100+frac. A scanned
// fraction carries no scale, so "10.5" — which is ten dollars fifty, the same
// amount Circle may write either way — came back as 1005 instead of 1050.
func TestParseAmount(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want currency.Cents
	}{
		{"10.50", 1050},
		{"10.5", 1050},
		{"10", 1000},
		{"0.01", 1},
		{"0", 0},
		{"-19.99", -1999},
		{"1234.56", 123456},
	} {
		got, err := parseAmount(tc.in)
		if err != nil {
			t.Errorf("parseAmount(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseAmount(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// An amount Circle sent that will not parse must be an error, not a zero. Zero
// is a legal amount, so mapping garbage to it reports a confident free payment.
func TestParseAmountUnreadableIsAnError(t *testing.T) {
	for _, in := range []string{"", "not-a-number", "USD 10.00"} {
		got, err := parseAmount(in)
		if err == nil {
			t.Errorf("parseAmount(%q) = %d with no error", in, got)
			continue
		}
		if !errors.Is(err, money.ErrInvalidAmount) {
			t.Errorf("parseAmount(%q) error = %v, want money.ErrInvalidAmount", in, err)
		}
	}
}

// What we send and what we read back are one conversion, so a Circle amount
// that leaves as a string must come home as the cents it left as.
func TestAmountRoundTrips(t *testing.T) {
	for _, cents := range []currency.Cents{0, 1, 99, 100, 1050, 123456, -1, -1999} {
		got, err := parseAmount(currency.USD.ToStringNoSymbol(cents))
		if err != nil {
			t.Errorf("round-trip %d: %v", cents, err)
			continue
		}
		if got != cents {
			t.Errorf("round-trip %d = %d", cents, got)
		}
	}
}
