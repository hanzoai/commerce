package processor

import "context"

// WalletConfigurer is a processor that keeps a commerce-owned configuration with the
// chosen wallets off and answers its id.
type WalletConfigurer interface {
	WalletConfiguration(ctx context.Context, applePay, googlePay bool) (string, error)
}

// LocalIntentMaker is a processor that opens PaymentIntents for the Payment Element.
type LocalIntentMaker interface {
	CreateLocalIntent(ctx context.Context, in LocalIntent) (*LocalIntentOut, error)
}

// LocalIntent is a PaymentIntent for the Payment Element: no payment method yet,
// no confirmation here. Stripe offers the buyer each method their country and
// the intent's currency allow (Pix and boleto in BRL, iDEAL and SEPA in EUR,
// OXXO in MXN, ...), as the account's payment method configuration enables them.
type LocalIntent struct {
	Amount   int64
	Currency string
	Key      string // idempotency key
	Metadata map[string]interface{}
	Desc     string
	// Excluded are payment method types the Payment Element must not offer.
	Excluded []string
	// Configuration is a Payment Method Configuration id to offer methods from, or "".
	Configuration string
	// Customer is the payer's customer at the processor; ReceiptEmail their email.
	Customer     string
	ReceiptEmail string
	// FXQuote is the processor's locked exchange rate (RateQuote.ID) the intent is
	// bound to, so the processor converts at the rate the buyer was quoted. "" for none.
	FXQuote string
}

// RateQuoter is a processor that quotes its own exchange rate to USD, the currency it
// settles in (Stripe's FX Quotes API), optionally locked for the life of one payment.
type RateQuoter interface {
	QuoteRate(ctx context.Context, cur, lock string) (*RateQuote, error)
}

// Rate locks: how long a quoted rate holds. LockNone is the current rate, no lock and
// no premium; the others bind a payment and carry the processor's lock premium.
const (
	LockNone = "none"
	LockHour = "hour"
	LockDay  = "day"
)

// RateQuote is a processor's rate for converting Currency to USD. PerDollar is what one
// dollar costs in Currency at the rate the processor will CONVERT at (its own
// conversion fee and lock premium included), in major units. BasePerDollar is the same
// before them. FeeRate and Premium are the fraction taken for each.
type RateQuote struct {
	ID            string
	Currency      string
	PerDollar     float64
	BasePerDollar float64
	FeeRate       float64
	Premium       float64
	// ExchangeRate is the same rate as PerDollar the way Stripe states it: USD per one
	// unit of Currency. ReferenceRate is the market reference Stripe measured against,
	// from ReferenceProvider.
	ExchangeRate      float64
	ReferenceRate     float64
	ReferenceProvider string
	Lock              string
	ExpiresAt         int64
}

// LocalIntentOut is what the browser needs to mount the Payment Element.
type LocalIntentOut struct {
	ID           string
	ClientSecret string
}

// IntentState is a PaymentIntent's state as the payment state machine names it.
func IntentState(status string) string {
	switch status {
	case "succeeded":
		return "succeeded"
	case "canceled":
		return "canceled"
	case "processing", "requires_action", "requires_confirmation", "requires_capture":
		return "pending"
	default: // requires_payment_method
		return "created"
	}
}
