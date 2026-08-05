package processor

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/money"
)

// A capture amount must name its own currency. When Capture took a bare
// currency.Cents the currency was absent from the signature, so every gateway
// guessed: adyen defaulted the label to "USD" and told callers to pass the real
// one "via options"; braintree and paypal rendered the amount with
// currency.USD.ToStringNoSymbol and labelled the body "USD".
//
// The scale is the part that costs money. A zero-decimal currency has no minor
// unit, so rendering 500 yen at USD's two decimals sends "5.00" — a capture of
// a hundredth of the authorization. This pins both halves: the digits and the
// label come from the amount, and nothing else may supply them.
func TestCaptureAmountCarriesItsCurrency(t *testing.T) {
	for _, c := range []struct {
		cur   currency.Type
		minor int64
		major string
		code  string
	}{
		{"usd", 1999, "19.99", "USD"},
		{"eur", 1050, "10.50", "EUR"},
		// The case the USD default broke: no minor unit, so the major string is
		// the whole number and dividing by 100 would send 5.
		{"jpy", 500, "500", "JPY"},
	} {
		amt := money.FromMinor(c.minor, c.cur.Money())
		if got := amt.MajorString(); got != c.major {
			t.Errorf("%d %s renders as %q, want %q", c.minor, c.cur, got, c.major)
		}
		if got := amt.Currency().Code; got != c.code {
			t.Errorf("%d %s is labelled %q, want %q", c.minor, c.cur, got, c.code)
		}
		if got := amt.Minor().Int64(); got != c.minor {
			t.Errorf("%d %s round-trips to %d minor units", c.minor, c.cur, got)
		}
	}
}

// Rendering a zero-decimal amount through USD is the exact defect the old code
// had. Pinned separately so the reason survives even if the table above changes.
func TestCaptureZeroDecimalIsNotRenderedAsUSD(t *testing.T) {
	yen := money.FromMinor(500, currency.Type("jpy").Money())
	if got := yen.MajorString(); got == "5.00" || got == "5" {
		t.Errorf("JPY 500 rendered as %q — that is a capture of 1/100th of the authorization", got)
	}
	if got := currency.USD.ToStringNoSymbol(500); got != "5.00" {
		t.Fatalf("precondition: USD 500 minor units should render 5.00, got %q", got)
	}
}
