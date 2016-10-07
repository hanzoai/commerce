package checkout

import "errors"

var (
	OrderDoesNotExist         = errors.New("Order does not exist")
	UserDoesNotExist          = errors.New("User does not exist")
	PaymentCancelled          = errors.New("Payment was cancelled")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body")
	FailedToCreateCustomer    = errors.New("Failed to create customer")
	FailedToCreateUser        = errors.New("Failed to create user")
	UnsupportedPaymentType    = errors.New("Unsupported payment type")
	OnlyOneOfUserBuyerAllowed = errors.New("Only one of user buyer allowed")
)
