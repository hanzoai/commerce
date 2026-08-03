package moonpay

import (
	"encoding/json"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

// baseCurrencyAmount arrives as a DECIMAL. Decoding it into a float64 and
// scaling by 100 lost a cent: 19.99*100 is 1998.9999999999998, which truncates
// to 1998. The outbound side already sent json.Number for this exact reason;
// this pins the inbound side to match.
func TestBaseCurrencyAmountDecodesExactly(t *testing.T) {
	for _, c := range []struct {
		body string
		want currency.Cents
	}{
		{`{"id":"t1","baseCurrencyAmount":19.99,"baseCurrencyCode":"usd"}`, 1999},
		{`{"id":"t2","baseCurrencyAmount":9.95,"baseCurrencyCode":"usd"}`, 995},
		{`{"id":"t3","baseCurrencyAmount":0.29,"baseCurrencyCode":"usd"}`, 29},
	} {
		var tx moonpayTransaction
		if err := json.Unmarshal([]byte(c.body), &tx); err != nil {
			t.Fatalf("unmarshal %s: %v", c.body, err)
		}
		got, err := currency.Type(tx.BaseCurrencyCode).Parse(tx.BaseCurrencyAmount.String())
		if err != nil {
			t.Errorf("amount %q: %v", tx.BaseCurrencyAmount, err)
			continue
		}
		if got != c.want {
			t.Errorf("baseCurrencyAmount %q = %d minor units, want %d", tx.BaseCurrencyAmount, got, c.want)
		}
	}
}

// An unreadable amount must never become a free transaction. json.Number
// refuses it at the DECODE layer, so it cannot reach the conversion at all — a
// float64 field accepted the same body and produced 0.
func TestBaseCurrencyAmountRefusesNonNumeric(t *testing.T) {
	var tx moonpayTransaction
	if err := json.Unmarshal([]byte(`{"id":"t4","baseCurrencyAmount":"nope","baseCurrencyCode":"usd"}`), &tx); err == nil {
		t.Errorf("a non-numeric amount decoded to %q; it must refuse rather than become 0", tx.BaseCurrencyAmount)
	}
}
