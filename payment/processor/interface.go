package processor

import (
	"context"
	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
)

// PaymentProcessor is the interface all payment processors must implement
type PaymentProcessor interface {
	// Type returns the processor type
	Type() ProcessorType

	// Charge processes a payment
	Charge(ctx context.Context, req PaymentRequest) (*PaymentResult, error)

	// Authorize authorizes a payment without capturing
	Authorize(ctx context.Context, req PaymentRequest) (*PaymentResult, error)

	// Capture captures a previously authorized payment
	// Capture takes a money.Amount, not a bare Cents, because a capture that
	// cannot name its currency is one every gateway has to guess at — and they
	// guessed USD, so a JPY authorization was captured at a hundredth of its
	// value and a EUR one went out labelled USD. Same reasoning, and the same
	// fix, as RefundRequest.Amount.
	Capture(ctx context.Context, transactionID string, amount money.Amount) (*PaymentResult, error)

	// Refund processes a refund
	Refund(ctx context.Context, req RefundRequest) (*RefundResult, error)

	// GetTransaction retrieves transaction details
	GetTransaction(ctx context.Context, txID string) (*Transaction, error)

	// ValidateWebhook validates an incoming webhook
	ValidateWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)

	// SupportedCurrencies returns currencies this processor supports
	SupportedCurrencies() []currency.Type

	// IsAvailable checks if processor is configured and available
	IsAvailable(ctx context.Context) bool
}

// CryptoProcessor extends PaymentProcessor with crypto-specific methods
type CryptoProcessor interface {
	PaymentProcessor

	// GenerateAddress mints a custody destination for a customer.
	//
	// It returns a Wallet rather than a bare address because an address alone
	// is only half a custody record: it says where funds land, not which wallet
	// the signer must be asked to spend from. Returning one value made losing
	// the other the DEFAULT — the MPC processor parsed wallet_id off the keygen
	// response and dropped it on the floor, so deposits could be credited and
	// never swept.
	GenerateAddress(ctx context.Context, customerID string, chain string) (Wallet, error)

	// GetBalance returns the balance for an address
	GetBalance(ctx context.Context, address string, chain string) (*Balance, error)

	// EstimateFee estimates transaction fees for a payment
	EstimateFee(ctx context.Context, req PaymentRequest) (currency.Cents, error)

	// SupportedChains returns the list of supported blockchain networks
	SupportedChains() []string
}

// SubscriptionProcessor extends PaymentProcessor with subscription methods
type SubscriptionProcessor interface {
	PaymentProcessor

	// CreateSubscription creates a recurring subscription
	CreateSubscription(ctx context.Context, req SubscriptionRequest) (*Subscription, error)

	// GetSubscription retrieves subscription details
	GetSubscription(ctx context.Context, subscriptionID string) (*Subscription, error)

	// CancelSubscription cancels a subscription
	CancelSubscription(ctx context.Context, subscriptionID string, immediately bool) error

	// UpdateSubscription modifies a subscription
	UpdateSubscription(ctx context.Context, subscriptionID string, req SubscriptionUpdate) (*Subscription, error)

	// ListSubscriptions lists subscriptions for a customer
	ListSubscriptions(ctx context.Context, customerID string) ([]*Subscription, error)
}

// CustomerProcessor extends PaymentProcessor with customer management
type CustomerProcessor interface {
	PaymentProcessor

	// CreateCustomer creates a customer in the processor
	CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error)

	// GetCustomer retrieves customer details
	GetCustomer(ctx context.Context, customerID string) (map[string]interface{}, error)

	// UpdateCustomer updates customer details
	UpdateCustomer(ctx context.Context, customerID string, updates map[string]interface{}) error

	// DeleteCustomer removes a customer
	DeleteCustomer(ctx context.Context, customerID string) error

	// AddPaymentMethod adds a payment method to a customer
	AddPaymentMethod(ctx context.Context, customerID, token string) (string, error)

	// RemovePaymentMethod removes a payment method
	RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error
}
