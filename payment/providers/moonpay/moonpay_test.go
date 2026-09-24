package moonpay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// configuredProvider points a real Provider at a test server by rewriting the
// request host, so the tests exercise the production path — struct tags, decode
// and conversion included — rather than a stand-in for it.
func configuredProvider(serverURL string) *Provider {
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.MoonPay, supportedCurrencies()),
		apiKey:        "pk_test",
		secretKey:     "sk_test",
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

// TestGetTransaction_AmountIsExact pins the INBOUND conversion on the real path,
// against the digits MoonPay actually puts on the wire.
//
// The field was float64 and the conversion was Cents(baseCurrencyAmount * 100).
// 19.99 has no exact binary form, so it decoded to 19.9899999… and the multiply
// truncated it to 1998 — a cent lost on an ordinary on-ramp purchase already
// paid for. Reading the field as json.Number keeps MoonPay's exact digits and
// money.ParseMinor converts them without ever touching a float.
func TestGetTransaction_AmountIsExact(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount string // exactly as it appears in MoonPay's JSON
		cur    string
		want   currency.Cents
	}{
		{"the case the float got wrong", "19.99", "usd", 1999},
		{"sub-dollar", "0.29", "usd", 29},
		{"another the float got wrong", "9.95", "usd", 995},
		{"whole dollars", "25.0", "usd", 2500},
		{"large amount keeps every digit", "1234567890.12", "usd", 123456789012},
		// MoonPay quotes some amounts and leaves others bare; json.Number reads
		// both, so the conversion does not depend on which we get.
		{"a quoted decimal reads the same", `"19.99"`, "usd", 1999},
		// A zero-decimal currency must NOT gain two: ¥500 is 500 minor units,
		// not 50000. The old code multiplied by a hardcoded 100 regardless.
		{"zero-decimal currency", "500", "jpy", 500},
		// The scale is looked up by code, so the code's case must not decide it.
		{"an upper-case code still scales", "500", "JPY", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":"tx-1","status":"completed","currencyCode":"eth",
					"baseCurrencyCode":%q,"baseCurrencyAmount":%s,"quoteCurrencyAmount":0.0052,
					"createdAt":"2026-01-15T10:00:00Z"}`, tc.cur, tc.amount)
			}))
			defer server.Close()

			tx, err := configuredProvider(server.URL).GetTransaction(context.Background(), "tx-1")
			if err != nil {
				t.Fatalf("GetTransaction: %v", err)
			}
			if tx.Amount != tc.want {
				t.Errorf("amount %s %s = %d minor units, want %d", tc.amount, tc.cur, tx.Amount, tc.want)
			}
		})
	}
}

// An amount MoonPay sent that will not parse must be an error, not a zero. Zero
// is a legal amount, so mapping garbage to it reports a confident free purchase
// and the spend silently vanishes from the ledger.
func TestGetTransaction_UnreadableAmountIsAnError(t *testing.T) {
	for _, tc := range []struct{ name, amount string }{
		{"not a number", `"not-a-number"`},
		{"absent", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":"tx-1","status":"completed","baseCurrencyCode":"usd",
					"baseCurrencyAmount":%s}`, tc.amount)
			}))
			defer server.Close()

			tx, err := configuredProvider(server.URL).GetTransaction(context.Background(), "tx-1")
			if err == nil {
				t.Fatalf("GetTransaction returned amount %d with no error", tx.Amount)
			}
		})
	}
}

// The crypto amount is reported straight to the caller as metadata. A crypto
// quantity carries far more significant digits than a float64 holds, so it
// keeps its exact digits too rather than being rounded on the way through.
func TestGetTransaction_CryptoAmountKeepsItsDigits(t *testing.T) {
	const quote = "0.00520000000000000001"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"tx-1","status":"completed","currencyCode":"eth",
			"baseCurrencyCode":"usd","baseCurrencyAmount":19.99,"quoteCurrencyAmount":%s}`, quote)
	}))
	defer server.Close()

	tx, err := configuredProvider(server.URL).GetTransaction(context.Background(), "tx-1")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	encoded, err := json.Marshal(tx.Metadata["crypto_amount"])
	if err != nil {
		t.Fatalf("marshal crypto_amount: %v", err)
	}
	if string(encoded) != quote {
		t.Errorf("crypto_amount = %s, want %s", encoded, quote)
	}
}
