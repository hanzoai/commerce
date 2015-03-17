package payment

import "errors"

var (
	OrderDoesNotExist         = errors.New("Order does not exist.")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body.")
	FailedToCreateCustomer    = errors.New("Failed to create customer.")
	FailedToCaptureCharge     = errors.New("Failed to capture charge.")
)
