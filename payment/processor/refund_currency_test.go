package processor

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/money"
)

// A refund amount must name its own currency. When RefundRequest carried a bare
// Cents, the currency was simply absent from the type, so five providers
// hardcoded USD on this path and a sixth divided by 100 — a JPY refund went out
// at 1/100th of its value, and every non-USD refund went out labelled USD.
//
// This pins the property the type now has: the scale and the label both come
// from the amount, so a provider cannot guess.
func TestRefundAmountCarriesItsCurrency(t *testing.T) {
	for _, c := range []struct {
		cur   currency.Type
		minor int64
		major string
		code  string
	}{
		{"usd", 1999, "19.99", "USD"},
		{"usd", 29, "0.29", "USD"},
		// Zero-decimal: the minor unit IS the major unit. Dividing by 100 here
		// is what sent a 500-yen refund as 5.00.
		{"jpy", 500, "500", "JPY"},
		{"eur", 1050, "10.50", "EUR"},
	} {
		req := RefundRequest{
			TransactionID: "t1",
			Amount:        money.FromMinor(c.minor, c.cur.Money()),
		}
		if got := req.Amount.MajorString(); got != c.major {
			t.Errorf("%d %s renders as %q, want %q", c.minor, c.cur, got, c.major)
		}
		if got := req.Amount.Currency().Code; got != c.code {
			t.Errorf("%d %s is labelled %q, want %q", c.minor, c.cur, got, c.code)
		}
		if got := req.Amount.Minor().Int64(); got != c.minor {
			t.Errorf("%d %s round-trips to %d minor units", c.minor, c.cur, got)
		}
	}
}

// The zero-value guard providers use must be the amount's own sign, not a
// comparison against an untyped int — a money.Amount has no ordering with 0.
func TestRefundAmountSignGuard(t *testing.T) {
	if (RefundRequest{Amount: money.FromMinor(0, currency.USD.Money())}).Amount.Sign() > 0 {
		t.Error("a zero refund must not pass the positive guard")
	}
	if (RefundRequest{Amount: money.FromMinor(-100, currency.USD.Money())}).Amount.Sign() > 0 {
		t.Error("a negative refund must not pass the positive guard")
	}
	if (RefundRequest{Amount: money.FromMinor(1, currency.USD.Money())}).Amount.Sign() <= 0 {
		t.Error("a one-minor-unit refund must pass the positive guard")
	}
}
