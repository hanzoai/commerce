package paymentmethod

import (
	"time"

	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/types"
)

// CardDetails holds card-specific information for a payment method.
type CardDetails struct {
	Brand    string `json:"brand"`
	Last4    string `json:"last4"`
	ExpMonth int    `json:"expMonth"`
	ExpYear  int    `json:"expYear"`
	Funding  string `json:"funding,omitempty"`
	Country  string `json:"country,omitempty"`
}

// BankAccountDetails holds US bank account information for a payment method.
type BankAccountDetails struct {
	BankName      string `json:"bankName,omitempty"`
	Last4         string `json:"last4"`
	RoutingNumber string `json:"routingNumber,omitempty"`
	AccountType   string `json:"accountType,omitempty"`
}

// PaymentMethod represents a customer's payment instrument.
type PaymentMethod struct {
	mixin.BaseModel

	UserId         string                 `json:"userId,omitempty"`
	CustomerId     string                 `json:"customerId,omitempty"`
	Name           string                 `json:"name,omitempty"`
	Type           string                 `json:"type"`
	ProviderRef    string                 `json:"providerRef"`
	ProviderType   string                 `json:"providerType"`
	Card           *CardDetails           `json:"card,omitempty"`
	BankAccount    *BankAccountDetails    `json:"bankAccount,omitempty"`
	BillingAddress *types.Address         `json:"billingAddress,omitempty"`
	IsDefault      bool                   `json:"isDefault,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Created        time.Time              `json:"created,omitempty"`
}
