// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package processor

import "errors"

// Decline is a card the processor refused, as the processor classified it: its
// error category and its error code, and nothing else.
//
// A processor's response to a refused card can carry the card itself: Square's
// includes the payment's card_details (brand, last four digits, expiry, BIN and
// fingerprint). None of that is kept here, and Error names only the code, so a
// Decline can be logged, wrapped and formatted without carrying card data.
//
// It is errors.Is ErrPaymentDeclined, so a caller that only asks whether the card
// said no keeps asking the question it already asks.
type Decline struct {
	Processor ProcessorType
	// Category is the processor's error category, e.g. Square's PAYMENT_METHOD_ERROR.
	Category string
	// Code is the processor's own code for the refusal, e.g. Square's CVV_FAILURE.
	// It is for operators; what a buyer is told is [Decline.Reason] and
	// [Decline.Sentence].
	Code string
}

func (d *Decline) Error() string { return "card declined (" + d.Code + ")" }

// Is reports a Decline as ErrPaymentDeclined.
func (d *Decline) Is(target error) bool { return target == ErrPaymentDeclined }

// DeclineOf finds the Decline in err's chain.
func DeclineOf(err error) (*Decline, bool) {
	var d *Decline
	if errors.As(err, &d) && d != nil {
		return d, true
	}
	return nil, false
}

// Decline reasons a buyer may be told. Each is the processor's own code for a
// refusal the processor itself explains to the buyer, and every other refusal is
// [ReasonDeclined]. Collapsing the rest is deliberate: a response that told apart
// an unknown card number, a lost card and a velocity limit would let a caller
// testing stolen cards learn from each attempt more than the card's own issuer
// tells its holder.
const (
	ReasonDeclined            = "CARD_DECLINED"
	ReasonCVV                 = "CVV_FAILURE"
	ReasonAddress             = "ADDRESS_VERIFICATION_FAILURE"
	ReasonExpired             = "EXPIRATION_FAILURE"
	ReasonInsufficientFunds   = "INSUFFICIENT_FUNDS"
	sentenceDeclined          = "Your card was declined by the bank."
	sentenceCVV               = "The security code (CVV) didn't match."
	sentenceAddress           = "The billing ZIP code didn't match."
	sentenceExpired           = "The card has expired."
	sentenceInsufficientFunds = "Insufficient funds."
)

// reasons maps a processor code onto the reason a buyer is told. A code not
// listed is [ReasonDeclined].
//
// Square names the same checks two ways: CVV_FAILURE and ADDRESS_VERIFICATION_FAILURE
// when a payment is taken, VERIFY_CVV_FAILURE and VERIFY_AVS_FAILURE when a card is
// vaulted. Both read the same to a buyer.
var reasons = map[string]string{
	"CVV_FAILURE":                  ReasonCVV,
	"VERIFY_CVV_FAILURE":           ReasonCVV,
	"ADDRESS_VERIFICATION_FAILURE": ReasonAddress,
	"VERIFY_AVS_FAILURE":           ReasonAddress,
	"EXPIRATION_FAILURE":           ReasonExpired,
	"CARD_EXPIRED":                 ReasonExpired,
	"INSUFFICIENT_FUNDS":           ReasonInsufficientFunds,
}

var sentences = map[string]string{
	ReasonDeclined:          sentenceDeclined,
	ReasonCVV:               sentenceCVV,
	ReasonAddress:           sentenceAddress,
	ReasonExpired:           sentenceExpired,
	ReasonInsufficientFunds: sentenceInsufficientFunds,
}

// Reason is the code a buyer is told: one of the Reason constants.
func (d *Decline) Reason() string {
	if r, ok := reasons[d.Code]; ok {
		return r
	}
	return ReasonDeclined
}

// Sentence is what a buyer is told about the refusal, in plain words.
func (d *Decline) Sentence() string { return sentences[d.Reason()] }
