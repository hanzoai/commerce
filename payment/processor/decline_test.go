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
