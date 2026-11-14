package stripe

import (
	"net/url"
	"strconv"
)

// IndiaMandate is the e-mandate an India-issued card is registered under for recurring
// charges (https://docs.stripe.com/india-recurring-payments): the RBI requires every
// mandate to state a maximum amount, and the cardholder to authenticate when it is set
// up. Max is in minor units of the payment's currency, sized by the caller to the plan
// plus headroom for the exchange rate; Interval is month, year, week, day or sporadic;
// Reference must be unique among the customer's active mandates.
type IndiaMandate struct {
	Max       int64
	Interval  string
	Count     int
	Reference string
	StartDate int64
}

// Params are the PaymentIntent parameters that register the mandate while the buyer is
// present, and ask for 3-D Secure so the first charge is authenticated. The currency is
// the intent's own, so it is not repeated here.
func (m IndiaMandate) Params() url.Values {
	v := url.Values{}
	const o = "payment_method_options[card][mandate_options]"
	v.Set(o+"[amount_type]", "maximum")
	v.Set(o+"[amount]", strconv.FormatInt(m.Max, 10))
	v.Set(o+"[interval]", m.Interval)
	if m.Interval != "sporadic" {
		v.Set(o+"[interval_count]", strconv.Itoa(max(1, m.Count)))
	}
	v.Set(o+"[reference]", m.Reference)
	v.Set(o+"[start_date]", strconv.FormatInt(m.StartDate, 10))
	v.Set(o+"[supported_types][]", "india")
	v.Set("payment_method_options[card][request_three_d_secure]", "any")
	v.Set("setup_future_usage", "off_session")
	return v
}

// IndiaNoAuthLimit is the RBI ceiling, in paise, for a recurring charge that may run
// without the cardholder authenticating it: Rs 15,000. Above it every charge needs the
// buyer, on-session.
const IndiaNoAuthLimit int64 = 15000 * 100
