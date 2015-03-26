package payment

import "errors"

var (
	OrderDoesNotExist         = errors.New("Order does not exist")
	UserDoesNotExist          = errors.New("User does not exist")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body")
	FailedToCreateCustomer    = errors.New("Failed to create customer")
	FailedToCreateUser        = errors.New("Failed to create user")
	FailedToCaptureCharge     = errors.New("Failed to capture charge")
	UnsupportedPaymentType    = errors.New("Unsupported payment type")
)
