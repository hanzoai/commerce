package billing

import (
	"errors"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// squareBody is text a Square refusal carries beside its code: the failed payment's
// card details, including a cvv_status field that says nothing about why it failed.
const squareBody = `{"payment":{"card_details":{"card":{"last_4":"1111"},"cvv_status":"CVV_ACCEPTED","avs_status":"AVS_ACCEPTED"}}}`

// TestParseCardDeclineReason_AClassifiedRefusalIsReadByItsCode — a refusal the
// processor classified is read by its code alone, never by the text beside it: every
// Square decline carries cvv_status, so read as text a plain decline would tell the
// buyer their security code was wrong.
func TestParseCardDeclineReason_AClassifiedRefusalIsReadByItsCode(t *testing.T) {
	for code, want := range map[string]string{
		"CARD_DECLINED":                "Your card was declined by the bank.",
		"CVV_FAILURE":                  "The security code (CVV) didn't match.",
		"ADDRESS_VERIFICATION_FAILURE": "The billing ZIP code didn't match.",
		"EXPIRATION_FAILURE":           "The card has expired.",
		"INSUFFICIENT_FUNDS":           "Insufficient funds.",
		"SOMETHING_NEW":                "Your card was declined by the bank.",
	} {
		d := &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: code}
		for name, res := range map[string]*processor.PaymentResult{
			"in the error":  nil,
			"in the result": {Error: d, ErrorMessage: squareBody},
		} {
			var err error = d
			if res != nil {
				err = errors.New(squareBody)
			}
			if got := parseCardDeclineReason(res, err); got != want {
				t.Errorf("%s, decline %s: %q, want %q", name, code, got, want)
			}
		}
	}
}

// TestTakePayment_ADeclinedCardAnswersItsSentenceAndCode — the token top-up's
// refusal is the buyer's sentence at 402, with the processor's decline beneath it for
// a caller that answers with the code.
//
// A failure that is not a refusal is the processor failing: 502, in a sentence of our
// own, never the processor's text.
func TestTakePayment_ADeclinedCardAnswersItsSentenceAndCode(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("declineco")

	m := squareMock("cust_d", "ccof_d", "")
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "INSUFFICIENT_FUNDS"}
	withFakeSquare(t, m)
	_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:a", AmountCents: 500, Subject: "declineco", IdempotencyKey: "d-1"})
	if f == nil || f.Status != 402 || f.Message != "Insufficient funds." {
		t.Fatalf("fault %+v, want 402 \"Insufficient funds.\"", f)
	}
	if d, ok := DeclineOf(f.Err); !ok || d.Code != "INSUFFICIENT_FUNDS" {
		t.Fatalf("the fault does not carry the decline: %v", f.Err)
	}

	m2 := squareMock("cust_e", "ccof_e", "")
	m2.chargeErr = errors.New("402: " + squareBody)
	withFakeSquare(t, m2)
	_, f = TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:b", AmountCents: 500, Subject: "declineco", IdempotencyKey: "d-2"})
	if f == nil || f.Status != 502 || f.Message != processorSentence || !IsProcessorFailed(f.Err) {
		t.Fatalf("fault %+v, want 502 %q", f, processorSentence)
	}
	if strings.Contains(f.Error(), "1111") || strings.Contains(f.Error(), "card_details") {
		t.Errorf("the fault carries the processor's text: %q", f.Error())
	}
}

// cardDeclined is Square's plain refusal, as commerce's Square processor answers it.
var cardDeclined = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CARD_DECLINED"}

// TestSubscribeCard_ADeclinedCardCarriesTheProcessorsCode — a plan sale's refusal
// carries the processor's decline beneath the buyer's sentence, so a caller can answer
// with Square's code, and it is still a declined sale.
func TestSubscribeCard_ADeclinedCardCarriesTheProcessorsCode(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-code")
	m := squareMock("cust_c", "ccof_c", "")
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CVV_FAILURE"}
	withFakeSquare(t, m)

	_, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:bad", PlanID: "dev", Subject: "sc-code", IdempotencyKey: "sc-code-1"})
	if !IsSaleDeclined(err) {
		t.Fatalf("err %v is not a declined sale", err)
	}
	if err.Error() != "The security code (CVV) didn't match." {
		t.Errorf("the buyer reads %q", err.Error())
	}
	if d, ok := DeclineOf(err); !ok || d.Code != "CVV_FAILURE" {
		t.Errorf("the sale does not carry the decline: %v", err)
	}
	if sub := parentSub(t, datastore.New(org.Namespaced(ctx)), "sc-code", "dev"); sub != nil {
		t.Error("a declined card opened a subscription")
	}
}
