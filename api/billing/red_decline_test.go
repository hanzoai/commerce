package billing

// red_decline_test.go — adversarial review of decline-reason (8af1d3de2). Each test
// drives commerce's REAL Square processor over the Square SDK and its HTTP client,
// with only the transport stubbed, so what is asserted is what production does.

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// squareHTTP answers the Square SDK's requests by method and path prefix, and
// counts every round trip.
type squareHTTP struct {
	mu     sync.Mutex
	routes []squareRoute
	calls  []string
}

type squareRoute struct {
	method, prefix string
	status         int
	body           string
}

func (w *squareHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, r.Method+" "+r.URL.Path)
	for _, rt := range w.routes {
		if r.Method == rt.method && strings.HasPrefix(r.URL.Path, rt.prefix) {
			return &http.Response{
				StatusCode: rt.status,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(rt.body)),
				Request:    r,
			}, nil
		}
	}
	return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"NOT_FOUND"}]}`)), Request: r}, nil
}

// realSquareOver is commerce's own Square processor whose SDK traffic goes to w,
// installed as the org's processor for the life of the test.
func realSquareOver(t *testing.T, w *squareHTTP) *squarelib.SquareProcessor {
	t.Helper()
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = w
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })
	return sp
}

const (
	customerOK = `{"customer":{"id":"SQ_CUST_1"}}`
	cardOK     = `{"card":{"id":"ccof:1","card_brand":"VISA","last_4":"1111","exp_month":12,"exp_year":2030,"fingerprint":"sq-1-fp"}}`
)

// TestRed_ASquareFailureThatIsNotTheCardNeverReadsAsAnExpiredCard — a Square error
// outside PAYMENT_METHOD_ERROR still falls back to reading Square's raw text, and the
// word "expired" in it becomes "The card has expired." at 402. An expired merchant
// access token (every buyer, every card) and an expired single-use nonce are both
// told to the buyer as their card having expired, through the plan sale and through
// the card-save mapping (payment_methods.go:255).
func TestRed_ASquareFailureThatIsNotTheCardNeverReadsAsAnExpiredCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for i, tc := range []struct {
		name   string
		routes []squareRoute
	}{
		{"the merchant's access token expired", []squareRoute{
			{"POST", "/v2/customers", 401, `{"errors":[{"category":"AUTHENTICATION_ERROR","code":"ACCESS_TOKEN_EXPIRED","detail":"This access token has expired."}]}`},
		}},
		{"the single-use nonce expired", []squareRoute{
			{"POST", "/v2/customers/SQ_CUST_1/cards", 400, `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"CARD_TOKEN_EXPIRED","detail":"The card nonce has expired."}]}`},
			{"POST", "/v2/customers", 200, customerOK},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp := realSquareOver(t, &squareHTTP{routes: tc.routes})

			subject := "red-exp-" + string(rune('a'+i))
			_, err := SubscribeCard(ctx, moneyOrg(subject), SubscribeIn{SourceID: "cnon:x", PlanID: "dev", Subject: subject, IdempotencyKey: subject})
			if err != nil && err.Error() == "The card has expired." {
				t.Errorf("plan sale: the buyer reads %q (declined sale %v) for a Square failure that never looked at the card", err.Error(), IsSaleDeclined(err))
			}

			_, verr := attachSquareCardOnFile(ctx, sp, "", "b@red.test", subject, "cnon:x")
			if verr == nil {
				t.Fatal("vault answered no error")
			}
			if got := parseCardDeclineReason(&processor.PaymentResult{ErrorMessage: verr.Error()}, verr); got == "The card has expired." {
				t.Errorf("card save: the buyer reads %q for %v", got, verr)
			}
		})
	}
}

// TestRed_ACardOnFileVerificationFailureKeepsItsSentence — a card is vaulted before
// every fresh-card sale and every card save, and Square names a failed CVV or ZIP
// check there VERIFY_CVV_FAILURE / VERIFY_AVS_FAILURE. The closed set maps neither,
// so both now read "Your card was declined by the bank." — where the text reading
// this commit replaced found "cvv" and told the buyer their security code was wrong.
func TestRed_ACardOnFileVerificationFailureKeepsItsSentence(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for i, tc := range []struct{ code, want string }{
		{"VERIFY_CVV_FAILURE", "The security code (CVV) didn't match."},
		{"VERIFY_AVS_FAILURE", "The billing ZIP code didn't match."},
	} {
		t.Run(tc.code, func(t *testing.T) {
			realSquareOver(t, &squareHTTP{routes: []squareRoute{
				{"POST", "/v2/customers/SQ_CUST_1/cards", 402, `{"errors":[{"category":"PAYMENT_METHOD_ERROR","code":"` + tc.code + `","detail":"Card verification failed."}]}`},
				{"POST", "/v2/customers", 200, customerOK},
			}})
			subject := "red-verify-" + string(rune('a'+i))
			_, err := SubscribeCard(ctx, moneyOrg(subject), SubscribeIn{SourceID: "cnon:x", PlanID: "dev", Subject: subject, IdempotencyKey: subject})
			if err == nil || err.Error() != tc.want {
				t.Errorf("%s: the buyer reads %v, want %q", tc.code, err, tc.want)
			}
		})
	}
}

// TestRed_ARetryAfterADeclineIsNotTheDeclinedCharge — a buyer told "The security code
// (CVV) didn't match." fixes it and pays again. Both cores abandon their local guard
// on a decline so that retry is not wedged, but the gateway key is derived from the
// amount (top-up) or the plan (sale) in a 15-minute window, never the card, so the
// retry reaches Square under the declined attempt's key: Square answers
// IDEMPOTENCY_KEY_REUSED for the new card (this mock replays the first answer), the
// buyer's corrected card is never tried, and a fresh card vaulted for the sale is
// removed again as "declined". Holds whenever the caller sends no X-Idempotency-Key.
func TestRed_ARetryAfterADeclineIsNotTheDeclinedCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	cvv := &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CVV_FAILURE"}

	t.Run("token top-up", func(t *testing.T) {
		m := squareMock("cust_rt", "ccof_rt", "sqpay_rt")
		m.chargeErr = cvv
		withFakeSquare(t, m)
		org := moneyOrg("red-retry-topup")
		if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:wrong-cvv", AmountCents: 500, Subject: "red-retry-topup"}); f == nil {
			t.Fatal("precondition: the first attempt was not declined")
		}
		m.chargeErr = nil
		if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:right-cvv", AmountCents: 500, Subject: "red-retry-topup"}); f != nil {
			t.Errorf("the corrected card was refused as %d %q: the retry reached Square under the declined attempt's key", f.Status, f.Message)
		}
	})

	t.Run("plan sale", func(t *testing.T) {
		m := squareMock("cust_rs", "ccof_rs", "sqpay_rs")
		m.chargeErr = cvv
		withFakeSquare(t, m)
		org := moneyOrg("red-retry-sale")
		if _, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:wrong-cvv", PlanID: "dev", Subject: "red-retry-sale"}); !IsSaleDeclined(err) {
			t.Fatalf("precondition: the first attempt answered %v", err)
		}
		m.chargeErr = nil
		if _, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:right-cvv", PlanID: "dev", Subject: "red-retry-sale"}); err != nil {
			t.Errorf("the corrected card was refused as %q: the retry reached Square under the declined attempt's key", err)
		}
	})
}
