package square

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/square/square-go-sdk/v3/core"
	"github.com/square/square-go-sdk/v3/option"
	"github.com/square/square-go-sdk/v3/payments"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// refusal is Square's CreatePayment answer to a refused card: the error list, and
// the failed payment beside it with the card's details.
func refusal(code string) string {
	return `{"errors":[{"code":"` + code + `","detail":"Authorization error: '` + code + `'","category":"PAYMENT_METHOD_ERROR"}],` +
		`"payment":{"id":"pay_failed_1","status":"FAILED","card_details":{"status":"FAILED",` +
		`"card":{"card_brand":"VISA","last_4":"1111","exp_month":12,"exp_year":2030,` +
		`"fingerprint":"sq-1-card-fingerprint","bin":"411111"},"cvv_status":"CVV_ACCEPTED","avs_status":"AVS_ACCEPTED"}}}`
}

// squareAnswering is a Square processor whose payments client talks to a server
// that answers every request with status and body.
func squareAnswering(t *testing.T, status int, body string) *SquareProcessor {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	sp := NewProcessor(Config{AccessToken: "test", LocationID: "loc_1", Environment: "sandbox"})
	sp.paymentsClient = payments.NewClient(core.NewRequestOptions(option.WithToken("test"), option.WithBaseURL(srv.URL)))
	return sp
}

var charge = processor.PaymentRequest{Token: "cnon:card-nonce-ok", Amount: currency.Cents(500), Currency: currency.USD}

// TestCharge_ARefusedCardIsADeclineWithSquaresCode is the lost code: Square's
// refusal reached the buyer as "charge failed" because the SDK hands back its whole
// response as one string. The charge now answers a processor.Decline carrying the
// category and the code, over the real SDK and HTTP path.
//
// And the response's card details go nowhere: the Decline, the result's message and
// every formatting of the error are free of the brand, last four, BIN, expiry and
// fingerprint Square returned beside the refusal.
func TestCharge_ARefusedCardIsADeclineWithSquaresCode(t *testing.T) {
	for _, code := range []string{"CARD_DECLINED", "CVV_FAILURE", "ADDRESS_VERIFICATION_FAILURE", "INSUFFICIENT_FUNDS", "EXPIRATION_FAILURE"} {
		for name, call := range map[string]func(*SquareProcessor) (*processor.PaymentResult, error){
			"charge": func(sp *SquareProcessor) (*processor.PaymentResult, error) {
				return sp.Charge(context.Background(), charge)
			},
			"authorize": func(sp *SquareProcessor) (*processor.PaymentResult, error) {
				return sp.Authorize(context.Background(), charge)
			},
		} {
			res, err := call(squareAnswering(t, http.StatusPaymentRequired, refusal(code)))
			d, ok := processor.DeclineOf(err)
			if !ok {
				t.Fatalf("%s %s: error %v is not a Decline", name, code, err)
			}
			if d.Code != code || d.Category != "PAYMENT_METHOD_ERROR" || d.Processor != processor.Square {
				t.Errorf("%s %s: decline %+v", name, code, d)
			}
			if res == nil || res.Success {
				t.Fatalf("%s %s: result %+v, want a failed result", name, code, res)
			}
			for _, text := range []string{err.Error(), res.ErrorMessage, res.Error.Error()} {
				for _, leak := range []string{"1111", "411111", "fingerprint", "VISA", "2030", "card_details"} {
					if strings.Contains(text, leak) {
						t.Errorf("%s %s: %q carries the card's %q", name, code, text, leak)
					}
				}
			}
		}
	}
}

// TestCharge_AFailureThatIsNotARefusalIsScrubbed — an authentication failure, a
// malformed request or an outage is Square failing, not the card. It is not a Decline,
// and it leaves this package as a processor.PaymentError naming Square's status,
// category and code, with none of Square's own text. That text can say anything
// ("This access token has expired.") and read as a reason it would tell a buyer
// something untrue about their card.
func TestCharge_AFailureThatIsNotARefusalIsScrubbed(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		code   string
	}{
		{http.StatusUnauthorized, `{"errors":[{"category":"AUTHENTICATION_ERROR","code":"ACCESS_TOKEN_EXPIRED","detail":"This access token has expired."}]}`, "ACCESS_TOKEN_EXPIRED"},
		{http.StatusBadRequest, `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"CARD_TOKEN_USED","detail":"used"}]}`, "CARD_TOKEN_USED"},
		{http.StatusInternalServerError, `not json`, "HTTP_500"},
	} {
		_, err := squareAnswering(t, tc.status, tc.body).Charge(context.Background(), charge)
		if err == nil {
			t.Fatalf("%d: no error", tc.status)
		}
		if _, ok := processor.DeclineOf(err); ok {
			t.Errorf("%d %s: read as a card refusal", tc.status, tc.body)
		}
		var pe *processor.PaymentError
		if !errors.As(err, &pe) || pe.Code != tc.code || pe.Processor != processor.Square {
			t.Errorf("%d: %v is not the scrubbed error with code %s", tc.status, err, tc.code)
		}
		var api *core.APIError
		if errors.As(err, &api) {
			t.Errorf("%d: the SDK's error, with Square's text, is still in the chain", tc.status)
		}
		for _, text := range []string{"expired", "used", "not json"} {
			if strings.Contains(err.Error(), text) {
				t.Errorf("%d: %q carries Square's text %q", tc.status, err.Error(), text)
			}
		}
	}
}

// TestCharge_ACardRefusalAmongOtherErrorsIsTheDecline — Square can list several errors
// for one request, and the card refusal is the one that decides the answer wherever it
// sits in the list.
func TestCharge_ACardRefusalAmongOtherErrorsIsTheDecline(t *testing.T) {
	body := `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"INVALID_VALUE"},` +
		`{"category":"PAYMENT_METHOD_ERROR","code":"CVV_FAILURE"}]}`
	_, err := squareAnswering(t, http.StatusPaymentRequired, body).Charge(context.Background(), charge)
	d, ok := processor.DeclineOf(err)
	if !ok || d.Code != "CVV_FAILURE" {
		t.Fatalf("a refusal listed second answered %v", err)
	}
}

// TestCharge_APendingPaymentIsStillProcessing — a payment Square answered 200 and has
// not settled is still processing, not declined.
func TestCharge_APendingPaymentIsStillProcessing(t *testing.T) {
	res, err := squareAnswering(t, http.StatusOK, `{"payment":{"id":"P1","status":"PENDING"}}`).Charge(context.Background(), charge)
	if err != nil || res == nil || res.Success {
		t.Fatalf("answered %+v, %v", res, err)
	}
	d, ok := processor.DeclineOf(res.Error)
	if !ok || !d.Processing() {
		t.Fatalf("a PENDING payment is %v, want still processing", res.Error)
	}
}
