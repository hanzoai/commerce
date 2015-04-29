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
	Param   string
}

func (e Error) Error() string {
	return e.Message
}

func NewError(err error) error {
	stripeErr, ok := err.(*stripe.Error)
	if ok {
		return &Error{
			Type:    string(stripeErr.Type),
			Message: stripeErr.Msg,
			Code:    string(stripeErr.Code),
			Param:   string(stripeErr.Param),
		}
	}

	return &Error{Type: "unknown", Message: fmt.Sprintf("Stripe error: %v", err)}
}
