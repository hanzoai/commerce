package bitpay

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

// The amount BitPay sends is a DECIMAL. Decoding it into a float64 and scaling
// by 100 lost a cent on ordinary prices — 19.99 has no exact binary form, so
// 19.99*100 is 1998.9999999999998 and truncating gives 1998, on money that was
// already captured. These drive the real wire struct and the real conversion.
func TestInvoicePriceDecodesExactly(t *testing.T) {
	for _, c := range []struct {
		body string
		cur  currency.Type
		want currency.Cents
	}{
		{`{"id":"i1","status":"complete","price":19.99,"currency":"USD"}`, "usd", 1999},
		{`{"id":"i2","status":"complete","price":9.95,"currency":"USD"}`, "usd", 995},
		{`{"id":"i3","status":"complete","price":0.29,"currency":"USD"}`, "usd", 29},
		{`{"id":"i4","status":"complete","price":1.15,"currency":"USD"}`, "usd", 115},
		// A refund/credit keeps its sign.
		{`{"id":"i5","status":"complete","price":-19.99,"currency":"USD"}`, "usd", -1999},
		// Zero-decimal: the minor unit IS the major unit, so 500 yen is 500.
		{`{"id":"i6","status":"complete","price":500,"currency":"JPY"}`, "jpy", 500},
	} {
		var d invoiceData
		if err := json.Unmarshal([]byte(c.body), &d); err != nil {
			t.Fatalf("unmarshal %s: %v", c.body, err)
		}
		got, err := c.cur.Parse(d.Price.String())
		if err != nil {
			t.Errorf("price %q: %v", d.Price, err)
			continue
		}
		if got != c.want {
			t.Errorf("price %q in %s = %d minor units, want %d", d.Price, c.cur, got, c.want)
		}
	}
}

// An unreadable amount must never become a silent zero — a zero-value
// transaction reports success while moving no money. json.Number refuses it at
// the DECODE layer, which is stronger than catching it at the conversion: the
// value cannot reach the conversion at all. A float64 field accepted the same
// body happily and produced 0.
func TestInvoicePriceRefusesNonNumeric(t *testing.T) {
	var d invoiceData
	if err := json.Unmarshal([]byte(`{"id":"i7","price":"not-a-number","currency":"USD"}`), &d); err == nil {
		t.Errorf("a non-numeric price decoded to %q; it must refuse rather than become 0", d.Price)
	}
}

// The outbound price must render at the CURRENCY's scale. float64(amount)/100
// assumed two decimals, so a zero-decimal currency was divided by 100 anyway.
func TestOutboundPriceRendersAtCurrencyScale(t *testing.T) {
	for _, c := range []struct {
		cur    currency.Type
		amount currency.Cents
		want   string
	}{
		{"usd", 1999, "19.99"},
		{"usd", 29, "0.29"},
		{"jpy", 500, "500"},
	} {
		body := invoiceRequest{Price: json.Number(c.cur.ToStringNoSymbol(c.amount))}
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(b), `"price":`+c.want+`,`) {
			t.Errorf("%d %s went out as %s, want price %s", c.amount, c.cur, b, c.want)
		}
	}
}
