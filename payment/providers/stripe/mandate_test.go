package stripe

import "testing"

func TestIndiaMandateParamsStateAMaximumAndAskForAuthentication(t *testing.T) {
	v := IndiaMandate{Max: 2200000, Interval: "month", Count: 1, Reference: "sub_1:2026-11", StartDate: 1795000000}.Params()
	for k, want := range map[string]string{
		"payment_method_options[card][mandate_options][amount_type]":       "maximum",
		"payment_method_options[card][mandate_options][amount]":            "2200000",
		"payment_method_options[card][mandate_options][interval]":          "month",
		"payment_method_options[card][mandate_options][interval_count]":    "1",
		"payment_method_options[card][mandate_options][reference]":         "sub_1:2026-11",
		"payment_method_options[card][mandate_options][start_date]":        "1795000000",
		"payment_method_options[card][mandate_options][supported_types][]": "india",
		"payment_method_options[card][request_three_d_secure]":             "any",
		"setup_future_usage": "off_session",
	} {
		if got := v.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if v.Get("payment_method_options[card][mandate_options][currency]") != "" {
		t.Error("a PaymentIntent mandate takes the intent's currency, not its own")
	}
	if s := (IndiaMandate{Max: 1, Interval: "sporadic", Reference: "r"}).Params(); s.Get("payment_method_options[card][mandate_options][interval_count]") != "" {
		t.Error("a sporadic mandate has no interval count")
	}
}
