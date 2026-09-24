package types

import (
	"encoding/json"
	"testing"

	"github.com/hanzoai/money"
)

// TestPricingTotalDecodesExactly pins the fulfillment cost Shipwire reports.
//
// The field was float64 and the caller did currency.Cents(total * 100). 9.95
// has no exact binary form, so it decoded to 9.9499999… and the multiply
// truncated it to 994 — a cent lost on every ordinary shipping charge, on a
// cost already incurred. Reading the field as json.Number keeps Shipwire's
// exact digits and money.ParseCents converts them without touching a float.
//
// Shipwire prices in USD at two decimals, so ParseCents is the right scale.
func TestPricingTotalDecodesExactly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		total string // exactly as it appears in Shipwire's JSON
		want  int64
	}{
		{"the case the float got wrong", "9.95", 995},
		{"another the float got wrong", "19.99", 1999},
		{"sub-dollar", "0.29", 29},
		{"whole dollars", "12", 1200},
		{"a quoted decimal reads the same", `"9.95"`, 995},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"pricing":{"resource":{"total":` + tc.total + `,"shipping":1.5}},
				"pricingEstimate":{"resource":{"total":` + tc.total + `}}}`)

			var o Order
			if err := json.Unmarshal(raw, &o); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for label, got := range map[string]json.Number{
				"pricing":         o.Pricing.Resource.Total,
				"pricingEstimate": o.PricingEstimate.Resource.Total,
			} {
				cents, err := money.ParseCents(got.String())
				if err != nil {
					t.Fatalf("%s: ParseCents(%q): %v", label, got, err)
				}
				if cents != tc.want {
					t.Errorf("%s total %s = %d cents, want %d", label, tc.total, cents, tc.want)
				}
			}
		})
	}
}
