package paymentmethods

type Type string

const (
	PlaidType Type = "plaid"
)

// Union object for all payment method parameters
type PaymentMethodParams struct {
	// Short lived public token reference
	PublicToken string
}

// Returned Pay Token
type PaymentMethodOutput struct {
	// Long lived payment token
	PayToken string

	// Reference to external token
	PayTokenId string

	// Reference to external user (if any)
	// ExternalUserId string

	// Type of payment method
	Type Type
}
