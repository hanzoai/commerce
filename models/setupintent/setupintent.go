package setupintent

import (
	"fmt"
	"time"

	"github.com/hanzoai/commerce/models/mixin"
)

// Status represents the state of a SetupIntent.
type Status string

const (
	RequiresPaymentMethod Status = "requires_payment_method"
	RequiresConfirmation  Status = "requires_confirmation"
	RequiresAction        Status = "requires_action"
	Processing            Status = "processing"
	Succeeded             Status = "succeeded"
	Canceled              Status = "canceled"
)

// SetupIntent represents a flow to save a payment method for future use.
type SetupIntent struct {
	mixin.BaseModel

	CustomerId         string                 `json:"customerId,omitempty"`
	PaymentMethodId    string                 `json:"paymentMethodId,omitempty"`
	Status             Status                 `json:"status"`
	Usage              string                 `json:"usage,omitempty"`
	ProviderRef        string                 `json:"providerRef"`
	ProviderType       string                 `json:"providerType"`
	CanceledAt         time.Time              `json:"canceledAt,omitempty"`
	CancellationReason string                 `json:"cancellationReason,omitempty"`
	LastError          string                 `json:"lastError,omitempty"`
	ClientSecret       string                 `json:"clientSecret,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// Confirm transitions the setup intent to Processing.
func (si *SetupIntent) Confirm() error {
	if si.Status != RequiresConfirmation && si.Status != RequiresPaymentMethod {
		return fmt.Errorf("cannot confirm setup intent in status %s", si.Status)
	}
	if si.PaymentMethodId == "" {
		return fmt.Errorf("payment method is required to confirm")
	}
	si.Status = Processing
	return nil
}

// MarkSucceeded marks the setup intent as succeeded.
func (si *SetupIntent) MarkSucceeded(providerRef string) {
	si.Status = Succeeded
	si.ProviderRef = providerRef
}

// Cancel transitions the setup intent to Canceled.
func (si *SetupIntent) Cancel(reason string) error {
	if si.Status == Succeeded || si.Status == Canceled {
		return fmt.Errorf("cannot cancel setup intent in status %s", si.Status)
	}
	si.Status = Canceled
	si.CancellationReason = reason
	si.CanceledAt = time.Now()
	return nil
}
