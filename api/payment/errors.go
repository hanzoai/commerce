package payment

import "errors"

var (
	OrderDoesNotExist         = errors.New("Order does not exist.")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body.")
	AuthorizationFailed       = errors.New("Authorization failed.")
	FailedToCreateCustomer    = errors.New("Failed to create customer.")
)
