package routing_test

import (
	"math"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/stripe"
	"github.com/hanzoai/commerce/payment/routing"
)

// Currencies in Stripe's presentment list that no country defaults to, and why.
var unreachable = map[string]string{
	"rub": "Russia is sanctioned (processor.Sanctioned)",
	"byn": "Belarus is sanctioned (processor.Sanctioned)",
	"ang": "retired: Curacao and Sint Maarten moved to xcg",
	"std": "retired: replaced by stn, and no rate source quotes it",
}

func TestEveryStripeCurrencyIsReachableAndNoCodeIsUnknown(t *testing.T) {
	presented := map[string]bool{}
	for _, c := range stripe.Presentment {
		presented[string(c)] = true
	}
	if len(presented) != len(stripe.Presentment) {
		t.Fatal("Presentment repeats a currency")
	}
	reached := map[string]bool{"usd": true}
	for _, code := range strings.Fields(routingISO(t)) {
		cur := routing.DefaultLocalCurrency(code)
		if cur != "usd" && !presented[cur] {
			t.Errorf("%s defaults to %q, which Stripe does not present", code, cur)
		}
		reached[cur] = true
		if processor.Sanctioned(code) && cur != "usd" {
			t.Errorf("sanctioned %s defaults to %q", code, cur)
		}
	}
	for c := range presented {
		if !reached[c] {
			if _, ok := unreachable[c]; !ok {
				t.Errorf("Stripe presents %s but no country defaults to it", c)
			}
		}
	}
	for c := range unreachable {
		if reached[c] {
			t.Errorf("%s is listed unreachable but a country reaches it", c)
		}
	}
	// Every country with a table entry is a real country.
	for _, code := range []string{"IN", "KR", "CN", "JP", "BR", "MX", "GB", "AU", "CA", "AE", "ZA", "ID", "VN", "TR", "ES", "FR", "IE", "BG"} {
		if !routing.KnownCountry(code) {
			t.Errorf("%s is not an ISO country", code)
		}
	}
}

func routingISO(t *testing.T) string {
	t.Helper()
	// KnownCountry is the one authority on the ISO list; walk every pair.
	var out []string
	for a := 'A'; a <= 'Z'; a++ {
		for b := 'A'; b <= 'Z'; b++ {
			if c := string([]rune{a, b}); routing.KnownCountry(c) {
				out = append(out, c)
			}
		}
	}
	return strings.Join(out, " ")
}

func TestCountriesLandOnTheirOwnMoney(t *testing.T) {
	want := map[string]string{"IN": "inr", "KR": "krw", "CN": "cny", "JP": "jpy", "BR": "brl", "MX": "mxn", "GB": "gbp",
		"AU": "aud", "CA": "cad", "AE": "aed", "ZA": "zar", "ID": "idr", "VN": "vnd", "TR": "try", "DE": "eur", "FR": "eur",
		"ES": "eur", "PL": "pln", "US": "usd", "RU": "usd", "IR": "usd", "KW": "usd", "PA": "pab"}
	for c, cur := range want {
		if got := routing.Default().LocalCurrency(c); got != cur {
			t.Errorf("%s = %s, want %s", c, got, cur)
		}
	}
}

func TestTheAdminOverrideBeatsTheTable(t *testing.T) {
	p := routing.Default()
	p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "IN", Currency: "usd"}, {Country: "US", Currency: "inr"}, {Country: "JP", Currency: "eur"}}
	for c, want := range map[string]string{"IN": "usd", "US": "inr", "JP": "eur", "KR": "krw"} {
		if got := p.LocalCurrency(" " + strings.ToLower(c)); got != want {
			t.Errorf("%s = %s, want %s", c, got, want)
		}
	}
}

func TestMethodsAreTiedToTheirCurrency(t *testing.T) {
	for m, cur := range map[string]string{"upi": "inr", "kakao_pay": "krw", "naver_pay": "krw", "payco": "krw", "samsung_pay": "krw", "kr_card": "krw", "wechat_pay": "cny", "pix": "brl"} {
		if !routing.MethodViable(m, cur) {
			t.Errorf("%s should be payable in %s", m, cur)
		}
		if !routing.KnownMethod(m) {
			t.Errorf("%s is not switchable", m)
		}
	}
	if routing.MethodViable("upi", "usd") || routing.MethodViable("kakao_pay", "jpy") || !routing.MethodViable("wechat_pay", "usd") {
		t.Error("method currencies are wrong")
	}
	for _, m := range routing.Excludable {
		if !routing.KnownMethod(m) {
			t.Errorf("excludable %s is unknown", m)
		}
	}
}

// A price is the smallest multiple of the currency's increment that covers what the
// dollars convert to: never under it, and less than one increment over it. Checked for $5
// and $8 in every currency Stripe presents, at rates from a few thousandths to tens of
// thousands. There is no minimum-charge clamp inside the price, so there is no exception:
// Stripe's minimum is checked on the dollars before a price is made.
func TestAPriceCoversWhatTheDollarsConvertToAndExceedsItByLessThanOneIncrement(t *testing.T) {
	p := routing.Default()
	rates := []float64{0.0031, 0.27, 0.74, 1.0, 1.368, 1.4429, 3.6725, 7.21, 83.5, 150.4, 1403.2, 16000.5, 25000}
	for i, c := range stripe.Presentment {
		cur := string(c)
		step := p.Step(cur)
		dec := currency.Type(cur).Decimals()
		for _, usd := range []int64{500, 800} {
			rate := rates[i%len(rates)]
			got, err := fx.Convert(usd, cur, rate, step)
			if err != nil {
				t.Errorf("%s $%d at %v: %v", cur, usd/100, rate, err)
				continue
			}
			scale := math.Pow10(int(dec))
			converted := float64(usd) / 100 * rate * scale // in minor units
			if float64(got) < converted-1e-6 {
				t.Errorf("%s $%d at %v: quoted %d minor units, under the %.4f it converts to", cur, usd/100, rate, got, converted)
			}
			if float64(got) >= converted+float64(step)-1e-6 {
				t.Errorf("%s $%d at %v: quoted %d minor units, a full increment (%d) or more over the %.4f it converts to", cur, usd/100, rate, got, step, converted)
			}
			if got%step != 0 {
				t.Errorf("%s: %d is not a multiple of its increment %d", cur, got, step)
			}
		}
	}
	// Canada, at today's rates: $5 at 1.4429 per dollar is C$7.21, so C$8: the smallest whole dollar over it.
	if got, _ := fx.Convert(500, "cad", 1.4429, p.Step("cad")); got != 800 {
		t.Errorf("$5 in CAD at 1.4429 = %d, want 800", got)
	}
	if got, _ := fx.Convert(500, "cad", 1.3683, p.Step("cad")); got != 700 {
		t.Errorf("$5 in CAD at 1.3683 (C$6.84) = %d, want 700", got)
	}
}
