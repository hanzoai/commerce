package processor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestDecline_EachCodeTellsTheBuyerItsSentence pins what a buyer reads for each
// refusal the processor explains, and that every other code — including one this
// package has never seen — reads as the generic decline, so a caller testing cards
// learns no more from a refusal than the card's issuer tells its holder.
func TestDecline_EachCodeTellsTheBuyerItsSentence(t *testing.T) {
	for _, tc := range []struct {
		code, reason, sentence string
	}{
		{"CARD_DECLINED", "CARD_DECLINED", "Your card was declined by the bank."},
		{"GENERIC_DECLINE", "CARD_DECLINED", "Your card was declined by the bank."},
		{"CVV_FAILURE", "CVV_FAILURE", "The security code (CVV) didn't match."},
		{"VERIFY_CVV_FAILURE", "CVV_FAILURE", "The security code (CVV) didn't match."},
		{"VERIFY_AVS_FAILURE", "ADDRESS_VERIFICATION_FAILURE", "The billing ZIP code didn't match."},
		{"ADDRESS_VERIFICATION_FAILURE", "ADDRESS_VERIFICATION_FAILURE", "The billing ZIP code didn't match."},
		{"EXPIRATION_FAILURE", "EXPIRATION_FAILURE", "The card has expired."},
		{"CARD_EXPIRED", "EXPIRATION_FAILURE", "The card has expired."},
		{"INSUFFICIENT_FUNDS", "INSUFFICIENT_FUNDS", "Insufficient funds."},
		{"PAN_FAILURE", "CARD_DECLINED", "Your card was declined by the bank."},
		{"CARD_DECLINED_CALL_ISSUER", "CARD_DECLINED", "Your card was declined by the bank."},
		{"TRANSACTION_LIMIT", "CARD_DECLINED", "Your card was declined by the bank."},
		{"A_CODE_NOBODY_HAS_SEEN", "CARD_DECLINED", "Your card was declined by the bank."},
		{"", "CARD_DECLINED", "Your card was declined by the bank."},
	} {
		d := &Decline{Processor: Square, Category: "PAYMENT_METHOD_ERROR", Code: tc.code}
		if got := d.Reason(); got != tc.reason {
			t.Errorf("%q: reason %q, want %q", tc.code, got, tc.reason)
		}
		if got := d.Sentence(); got != tc.sentence {
			t.Errorf("%q: sentence %q, want %q", tc.code, got, tc.sentence)
		}
	}
}

// TestDecline_IsFoundThroughWrappingAndCarriesNoCardText — a Decline is found
// wherever it is wrapped, answers as ErrPaymentDeclined, and formats as the code
// alone.
func TestDecline_IsFoundThroughWrappingAndCarriesNoCardText(t *testing.T) {
	d := &Decline{Processor: Square, Category: "PAYMENT_METHOD_ERROR", Code: "CVV_FAILURE"}
	wrapped := fmt.Errorf("charge failed: %w", d)

	got, ok := DeclineOf(wrapped)
	if !ok || got != d {
		t.Fatalf("DeclineOf(wrapped) = %v, %v; want the decline", got, ok)
	}
	if !errors.Is(wrapped, ErrPaymentDeclined) {
		t.Error("a wrapped Decline is not ErrPaymentDeclined")
	}
	if msg := wrapped.Error(); msg != "charge failed: card declined (CVV_FAILURE)" {
		t.Errorf("formatted as %q", msg)
	}
	if _, ok := DeclineOf(errors.New("charge failed")); ok {
		t.Error("an error with no Decline in it was read as one")
	}
	if _, ok := DeclineOf(nil); ok {
		t.Error("nil was read as a Decline")
	}
	if strings.Contains(wrapped.Error(), "PAYMENT_METHOD_ERROR") {
		t.Error("the category is for operators and should not be in the message")
	}
}

// TestDecline_APaymentStillProcessingIsNotARefusal — Square's PENDING and APPROVED
// are payments that may yet settle: their own reason and sentence, and
// ErrUnknownOutcome rather than ErrPaymentDeclined, so no caller retries them under a
// new key or counts them against a wallet. FAILED and CANCELED will not settle and are
// refusals.
func TestDecline_APaymentStillProcessingIsNotARefusal(t *testing.T) {
	for _, tc := range []struct {
		code       string
		processing bool
	}{{"PENDING", true}, {"APPROVED", true}, {"FAILED", false}, {"CANCELED", false}} {
		d := &Decline{Processor: Square, Category: StatusCategory, Code: tc.code}
		if d.Processing() != tc.processing {
			t.Errorf("%s: processing %t", tc.code, d.Processing())
		}
		if errors.Is(d, ErrUnknownOutcome) != tc.processing || errors.Is(d, ErrPaymentDeclined) == tc.processing {
			t.Errorf("%s: is unknown-outcome %t, is declined %t", tc.code, errors.Is(d, ErrUnknownOutcome), errors.Is(d, ErrPaymentDeclined))
		}
		wantReason, wantSentence := ReasonDeclined, "Your card was declined by the bank."
		if tc.processing {
			wantReason, wantSentence = ReasonProcessing, "Your payment is still processing. Please don't pay again."
		}
		if d.Reason() != wantReason || d.Sentence() != wantSentence {
			t.Errorf("%s: %s %q", tc.code, d.Reason(), d.Sentence())
		}
	}
	// A card refusal whose code happens to read PENDING is still a refusal: only the
	// status category names a payment's state.
	if (&Decline{Category: "PAYMENT_METHOD_ERROR", Code: "PENDING"}).Processing() {
		t.Error("a PAYMENT_METHOD_ERROR was read as a payment still processing")
	}
}

// TestRejected_OnlyTheBuyersRequest — a request the processor refused outside a card
// decline is a known outcome only when it is a client error that is not about the
// merchant's credentials, a rate limit or a reused key; anything the processor did
// not answer, or answered 5xx, is not.
func TestRejected_OnlyTheBuyersRequest(t *testing.T) {
	answered := func(status int, code string) error {
		pe := NewPaymentError(Square, code, "square answered", nil)
		pe.Status = status
		return fmt.Errorf("charge: %w", pe)
	}
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{answered(400, "CARD_TOKEN_USED"), true},
		{answered(404, "NOT_FOUND"), true},
		{answered(422, "INVALID_VALUE"), true},
		{answered(400, KeyReused), false},
		{answered(401, "ACCESS_TOKEN_EXPIRED"), false},
		{answered(403, "INSUFFICIENT_SCOPES"), false},
		{answered(429, "RATE_LIMITED"), false},
		{answered(500, "INTERNAL_SERVER_ERROR"), false},
		{answered(503, "SERVICE_UNAVAILABLE"), false},
		{answered(0, "HTTP_0"), false},
		{errors.New("dial tcp: connection refused"), false},
		{&Decline{Processor: Square, Category: "PAYMENT_METHOD_ERROR", Code: "CARD_DECLINED"}, false},
		{nil, false},
	} {
		if got := Rejected(tc.err); got != tc.want {
			t.Errorf("%v: rejected %v, want %v", tc.err, got, tc.want)
		}
	}
}
