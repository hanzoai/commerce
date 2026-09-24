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

// ErrUnknownOutcome marks a charge whose outcome the processor has not stated: it
// failed to answer, or it answered with a payment it has not settled. The money may
// yet move, so another attempt must reach the processor under the same idempotency
// key, where the processor answers it with the first payment instead of taking a
// second.
var ErrUnknownOutcome = errors.New("the charge's outcome is not known yet")

// Rejected reports whether err is the processor refusing the request itself rather
// than the card: a client error that is not about the merchant's own credentials
// (401, 403), a rate limit (429) or an idempotency key sent again with a different
// request. A spent or malformed token, or a saved card that no longer exists, is
// answered this way, and the processor takes no payment for it, so its outcome is
// known. A reused key is not, because a payment may exist under it.
func Rejected(err error) bool {
	var pe *PaymentError
	if !errors.As(err, &pe) || pe.Status < 400 || pe.Status >= 500 {
		return false
	}
	switch pe.Status {
	case 401, 403, 429:
		return false
	}
	return pe.Code != KeyReused
}

// KeyReused is Square's code for an idempotency key sent again with a different request.
const KeyReused = "IDEMPOTENCY_KEY_REUSED"

// StatusCategory is the category of a Decline built from a payment the processor
// answered and did not settle, named by the payment's status.
const StatusCategory = "PAYMENT_STATUS"

// Processing reports a payment the processor answered and has not settled, which
// may still settle: Square's PENDING (a hold, or a bank transfer clearing) and
// APPROVED (authorized, capture pending). It is not a refusal. It is errors.Is
// ErrUnknownOutcome and never ErrPaymentDeclined.
func (d *Decline) Processing() bool {
	return d.Category == StatusCategory && (d.Code == "PENDING" || d.Code == "APPROVED")
}

// Is reports a Decline as ErrPaymentDeclined, and a payment still processing as
// ErrUnknownOutcome.
func (d *Decline) Is(target error) bool {
	if d.Processing() {
		return target == ErrUnknownOutcome
	}
	return target == ErrPaymentDeclined
}

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
	ReasonProcessing          = "PAYMENT_PROCESSING"
	sentenceDeclined          = "Your card was declined by the bank."
	sentenceCVV               = "The security code (CVV) didn't match."
	sentenceAddress           = "The billing ZIP code didn't match."
	sentenceExpired           = "The card has expired."
	sentenceInsufficientFunds = "Insufficient funds."
	sentenceProcessing        = "Your payment is still processing. Please don't pay again."
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
	ReasonProcessing:        sentenceProcessing,
}

// Reason is the code a buyer is told: one of the Reason constants.
func (d *Decline) Reason() string {
	if d.Processing() {
		return ReasonProcessing
	}
	if r, ok := reasons[d.Code]; ok {
		return r
	}
	return ReasonDeclined
}

// Sentence is what a buyer is told about the refusal, in plain words.
func (d *Decline) Sentence() string { return sentences[d.Reason()] }
