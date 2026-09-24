package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"

	. "github.com/hanzoai/commerce/types"
)

// A webhook body arrives as map[string]interface{}, so every JSON number is a
// float64 by the time it reaches here. Whole minor-unit amounts — which is what
// every processor on this path sends — must survive exactly.
func TestSettlementAmountReadsWholeMinorUnits(t *testing.T) {
	for _, c := range []struct {
		name string
		data Map
		want currency.Cents
		cur  currency.Type
	}{
		{"square nested", Map{"amount_money": map[string]interface{}{"amount": float64(2500), "currency": "USD"}}, 2500, "usd"},
		{"square jpy", Map{"amount_money": map[string]interface{}{"amount": float64(500), "currency": "JPY"}}, 500, "jpy"},
		{"flat cents", Map{"amount": float64(1999), "currency": "USD"}, 1999, "usd"},
		{"int64 form", Map{"amount": int64(1999)}, 1999, currency.USD},
	} {
		got, cur := settlementAmount(c.data)
		if got != c.want {
			t.Errorf("%s: amount = %d, want %d", c.name, got, c.want)
		}
		if cur != c.cur {
			t.Errorf("%s: currency = %q, want %q", c.name, cur, c.cur)
		}
	}
}

// A FRACTIONAL amount is not cents-with-a-fraction; it means the payload states
// major units, so 19.99 is $19.99 and not 19 cents. Truncating it credited the
// customer 19 cents for a $19.99 payment. There is no safe guess between the two
// readings, so it must refuse — the caller turns a non-positive amount into
// "record the event, write no ledger row".
func TestSettlementAmountRefusesFractional(t *testing.T) {
	for _, c := range []struct {
		name string
		data Map
	}{
		{"flat major units", Map{"amount": 19.99, "currency": "USD"}},
		{"flat sub-cent", Map{"amount": 0.29}},
		{"nested major units", Map{"amount_money": map[string]interface{}{"amount": 19.99, "currency": "USD"}}},
	} {
		got, _ := settlementAmount(c.data)
		if got != 0 {
			t.Errorf("%s: amount = %d, want 0 — a fractional amount must not be truncated "+
				"into a ledger row (19.99 major units became 19 cents)", c.name, got)
		}
	}
}

// A missing amount must also be unreadable rather than a silent zero that looks
// like a legitimately zero payment.
func TestSettlementAmountRefusesMissing(t *testing.T) {
	if got, _ := settlementAmount(Map{"currency": "USD"}); got != 0 {
		t.Errorf("missing amount = %d, want 0", got)
	}
}
