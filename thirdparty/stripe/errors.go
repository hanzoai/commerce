package stripe

import (
	"errors"
	"fmt"

	"github.com/stripe/stripe-go"
)

var (
	FailedToCreateCustomer = errors.New("Failed to create Stripe customer.")
	FailedToUpdateCustomer = errors.New("Failed to update Stripe customer.")
)

type Error struct {
	Type    string
	Message string
	Code    string
}

func (e Error) Error() string {
	return e.Message
}

func NewError(err error) error {
	stripeErr, ok := err.(*stripe.Error)
	if ok {
		return &Error{
			Code:    string(stripeErr.Code),
			Message: stripeErr.Msg,
			Type:    string(stripeErr.Type),
		}
	}

	return &Error{Type: "unknown", Message: fmt.Sprintf("Stripe error: %v", err)}
}
