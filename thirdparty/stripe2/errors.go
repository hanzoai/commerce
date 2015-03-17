package stripe

import "errors"

var (
	FailedToCreateCustomer = errors.New("Failed to create Stripe customer.")
	FailedtoUpdateCustomer = errors.New("Failed to update Stripe customer.")
)

type Error struct {
	Type    string
	Message string
	Code    string
}

func (e Error) Error() string {
	return e.Message
}
