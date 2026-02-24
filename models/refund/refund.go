package refund

import (
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
)

// Status represents the state of a refund.
type Status string

const (
	Pending   Status = "pending"
	Succeeded Status = "succeeded"
	Failed    Status = "failed"
	Canceled  Status = "canceled"
)

// Refund represents a reversal of a previous payment.
type Refund struct {
	mixin.BaseModel

	Amount          int64                  `json:"amount"`
	Currency        currency.Type          `json:"currency"`
	Status          Status                 `json:"status"`
	ProviderRef     string                 `json:"providerRef"`
	Reason          string                 `json:"reason,omitempty"`
	ReceiptNumber   string                 `json:"receiptNumber,omitempty"`
	FailureReason   string                 `json:"failureReason,omitempty"`
	PaymentIntentId string                 `json:"paymentIntentId,omitempty"`
	InvoiceId       string                 `json:"invoiceId,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// MarkSucceeded transitions the refund to Succeeded.
func (r *Refund) MarkSucceeded() error {
	r.Status = Succeeded
	return nil
}

// MarkFailed transitions the refund to Failed with a reason.
func (r *Refund) MarkFailed(reason string) error {
	r.Status = Failed
	r.FailureReason = reason
	return nil
}
