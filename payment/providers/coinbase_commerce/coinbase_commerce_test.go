package coinbase_commerce

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

func configuredProvider(serverURL string) *Provider {
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.CoinbaseCommerce, supportedCurrencies()),
		apiKey:        "test-key",
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &rewriteTransport{base: http.DefaultTransport, targetURL: serverURL},
		},
	}
	p.SetConfigured(true)
	return p
}

type rewriteTransport struct {
	base      http.RoundTripper
	targetURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.targetURL, "http://")
	return t.base.RoundTrip(req)
}

// TestCharge_LocalPriceIsAnExactDecimal pins the OUTBOUND amount. local_price
// is a quoted decimal, so we can send the exact digits — but the old code built
// it with fmt.Sprintf("%.2f", float64(cents)/100), which routed the amount
// through a binary float and hardcoded two decimals. A zero-decimal currency
// was billed at a hundredth of the intended amount: ¥500 went out as "5.00".
func TestCharge_LocalPriceIsAnExactDecimal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cur    currency.Type
		amount currency.Cents
		want   string // exactly as it must appear in the JSON
	}{
		{"cent-precise", currency.USD, 1999, `"amount":"19.99"`},
		{"sub-dollar", currency.USD, 29, `"amount":"0.29"`},
		{"large amount keeps every digit", currency.USD, 123456789012, `"amount":"1234567890.12"`},
		// ¥500 must go out as 500, not 5.00.
		{"zero-decimal currency", currency.JPY, 500, `"amount":"500"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"data":{"id":"chg-1","code":"ABC","hosted_url":"https://x","timeline":[]}}`))
			}))
			defer server.Close()

			if _, err := configuredProvider(server.URL).Charge(context.Background(), processor.PaymentRequest{
				Amount:   tc.amount,
				Currency: tc.cur,
			}); err != nil {
				t.Fatalf("Charge: %v", err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Errorf("request body = %s, want it to contain %s", got, tc.want)
			}
		})
	}
}
