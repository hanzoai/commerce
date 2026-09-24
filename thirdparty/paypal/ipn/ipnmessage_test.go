package ipn

import (
	"net/url"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

// The IPN amount field is "<CURRENCY> <decimal>" — the currency is right there
// in the same field as the digits. Parsing the digits at a hardcoded USD scale
// ignored it, so a zero-decimal notification was recorded a hundred times too
// large: "JPY 500" is 500 minor units, not 50000.
func TestNewIpnMessage_AmountScalesByItsOwnCurrency(t *testing.T) {
	for _, tc := range []struct {
		name     string
		raw      string
		wantCur  currency.Type
		wantCent currency.Cents
	}{
		{"two-decimal currency is unchanged", "USD 19.99", "usd", 1999},
		{"sub-dollar", "USD 0.29", "usd", 29},
		{"the case the float family got wrong", "USD 9.95", "usd", 995},
		// A zero-decimal currency has no minor unit to scale into.
		{"zero-decimal currency", "JPY 500", "jpy", 500},
		{"a refund keeps its sign", "USD -19.99", "usd", -1999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := NewIpnMessage(url.Values{
				"transaction[0].amount": {tc.raw},
			})
			if err != nil {
				t.Fatalf("NewIpnMessage(%q): %v", tc.raw, err)
			}
			if msg.Currency != tc.wantCur {
				t.Errorf("currency = %q, want %q", msg.Currency, tc.wantCur)
			}
			if msg.Amount != tc.wantCent {
				t.Errorf("amount %q = %d minor units, want %d", tc.raw, msg.Amount, tc.wantCent)
			}
		})
	}
}

// An amount PayPal sent that will not parse must stay an error, not become a
// zero — a zero-amount IPN posts a free transaction that reports success.
func TestNewIpnMessage_UnreadableAmountIsAnError(t *testing.T) {
	for _, raw := range []string{"USD not-a-number", "19.99", "", "USD"} {
		if msg, err := NewIpnMessage(url.Values{
			"transaction[0].amount": {raw},
		}); err == nil {
			t.Errorf("NewIpnMessage(%q) returned amount %d with no error", raw, msg.Amount)
		}
	}
}
