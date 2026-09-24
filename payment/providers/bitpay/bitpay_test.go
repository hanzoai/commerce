package bitpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		BaseProcessor: processor.NewBaseProcessor(processor.BitPay, supportedCurrencies()),
		apiToken:      "test-token",
		environment:   "test",
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

// body reads the request body a test server received, so a test can assert on
// the exact bytes we put on the wire.
func body(t *testing.T, r *http.Request) []byte {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	return b
}

// signPayload produces the HMAC-SHA256 signature BitPay webhooks carry.
func signPayload(token string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestGetTransaction_AmountIsExact pins the INBOUND conversion on the real path,
// against the digits BitPay actually puts on the wire.
//
// The field was float64 and the conversion was currency.Cents(price * 100).
// 19.99 has no exact binary form, so it decoded to 19.9899999… and the multiply
// truncated to 1998 — a cent lost on an ordinary price, on an invoice already
// settled. Reading the field as json.Number keeps BitPay's exact digits and
// money.ParseMinor converts them without ever touching a float.
func TestGetTransaction_AmountIsExact(t *testing.T) {
	for _, tc := range []struct {
		name  string
		price string // exactly as it appears in BitPay's JSON
		cur   string
		want  currency.Cents
	}{
		{"the case the float got wrong", "19.99", "USD", 1999},
		{"sub-dollar", "0.29", "USD", 29},
		{"another the float got wrong", "9.95", "USD", 995},
		{"whole dollars", "25.0", "USD", 2500},
		{"large amount keeps every digit", "1234567890.12", "USD", 123456789012},
		// BitPay quotes some amounts and leaves others bare; json.Number reads
		// both, so the conversion does not depend on which we get.
		{"a quoted decimal reads the same", `"19.99"`, "USD", 1999},
		// A zero-decimal currency must NOT gain two: ¥500 is 500 minor units,
		// not 50000. The old code multiplied by a hardcoded 100 regardless.
		{"zero-decimal currency", "500", "JPY", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":{"id":"inv-1","url":"https://bitpay.com/i/inv-1",
					"status":"complete","price":%s,"currency":%q,"orderId":"ord-1",
					"invoiceTime":1700000000000,"currentTime":1700000001000}}`, tc.price, tc.cur)
			}))
			defer server.Close()

			tx, err := configuredProvider(server.URL).GetTransaction(context.Background(), "inv-1")
			if err != nil {
				t.Fatalf("GetTransaction: %v", err)
			}
			if tx.Amount != tc.want {
				t.Errorf("price %s %s = %d cents, want %d", tc.price, tc.cur, tx.Amount, tc.want)
			}
		})
	}
}

// A price BitPay sent that will not parse must be an error, not a zero. Zero is
// a legal amount, so mapping garbage to it reports a confident free invoice and
// the money silently leaves the ledger.
func TestGetTransaction_UnreadableAmountIsAnError(t *testing.T) {
	for _, tc := range []struct{ name, price string }{
		{"not a number", `"not-a-number"`},
		{"absent", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":{"id":"inv-1","status":"complete",
					"price":%s,"currency":"USD"}}`, tc.price)
			}))
			defer server.Close()

			tx, err := configuredProvider(server.URL).GetTransaction(context.Background(), "inv-1")
			if err == nil {
				t.Fatalf("GetTransaction returned amount %d with no error", tx.Amount)
			}
		})
	}
}

// TestCharge_PriceIsAnExactNumber pins the OUTBOUND half: the price we ask
// BitPay to invoice must carry the exact digits of the amount, at the scale its
// own currency uses. float64(cents)/100 divided by a hardcoded 100, so a
// zero-decimal currency was billed at a hundredth of the intended amount.
func TestCharge_PriceIsAnExactNumber(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cur    currency.Type
		amount currency.Cents
		want   string // exactly as it must appear in the JSON
	}{
		{"cent-precise", currency.USD, 1999, `"price":19.99`},
		{"sub-dollar", currency.USD, 29, `"price":0.29`},
		{"large amount keeps every digit", currency.USD, 123456789012, `"price":1234567890.12`},
		// ¥500 must go out as 500, not 5.
		{"zero-decimal currency", currency.JPY, 500, `"price":500`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = body(t, r)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"id":"inv-1","status":"new","price":1,"currency":"USD"}}`)
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

// TestRefund_AmountIsAnExactNumber pins the same for the refund we send. A
// refund is money leaving on our instruction, so an amount off by a cent here
// is a real over- or under-refund.
//
// RefundRequest.Amount now carries its own currency, so the refund renders at
// that currency's scale rather than an assumed two decimals.
func TestRefund_AmountIsAnExactNumber(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cur    currency.Type
		amount currency.Cents
		want   string
	}{
		{"cent-precise", currency.USD, 1999, `"amount":19.99`},
		{"sub-dollar", currency.USD, 29, `"amount":0.29`},
		{"large amount keeps every digit", currency.USD, 123456789012, `"amount":1234567890.12`},
		// A refund of a zero-decimal charge must not be sent at a hundredth of
		// its value.
		{"zero-decimal currency", currency.JPY, 500, `"amount":500`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = body(t, r)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"id":"ref-1","status":"created"}}`)
			}))
			defer server.Close()

			if _, err := configuredProvider(server.URL).Refund(context.Background(), processor.RefundRequest{
				TransactionID: "inv-1",
				Amount:        tc.cur.Amount(tc.amount),
			}); err != nil {
				t.Fatalf("Refund: %v", err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Errorf("request body = %s, want it to contain %s", got, tc.want)
			}
		})
	}
}

// The webhook carries the same price field, so it must reach the event data
// with its digits intact for anything downstream that reads it.
func TestValidateWebhook_PriceKeepsItsDigits(t *testing.T) {
	p := configuredProvider("http://unused")
	payload := []byte(`{"event":{"name":"invoice_completed"},
		"data":{"id":"inv-1","status":"complete","price":19.99,"currency":"USD","orderId":"ord-1"}}`)

	evt, err := p.ValidateWebhook(context.Background(), payload, signPayload(p.apiToken, payload))
	if err != nil {
		t.Fatalf("ValidateWebhook: %v", err)
	}
	encoded, err := json.Marshal(evt.Data["price"])
	if err != nil {
		t.Fatalf("marshal price: %v", err)
	}
	if string(encoded) != "19.99" {
		t.Errorf("webhook price = %s, want 19.99", encoded)
	}
}
