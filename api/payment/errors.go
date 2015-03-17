package payment

import (
	"errors"
	"fmt"
)

var (
	OrderDoesNotExist         = errors.New("Order does not exist.")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body.")
	FailedToCreateCustomer    = errors.New("Failed to create customer.")
)

type AuthorizationFailed struct {
	Type    string
	Message string
	Code    string
}

func (a AuthorizationFailed) Error() string {
	return fmt.Sprintf("Authorization failed: %s", a.Message)
}
