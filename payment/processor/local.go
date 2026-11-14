package processor

import "context"

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
	// Customer is the payer's customer at the processor; ReceiptEmail their email.
	Customer     string
	ReceiptEmail string
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
