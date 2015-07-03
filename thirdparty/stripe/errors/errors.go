package errors

import (
	"errors"
	"fmt"

	"github.com/stripe/stripe-go"
)

var (
	FailedToCreateCustomer = errors.New("Failed to create Stripe customer.")
	FailedToUpdateCustomer = errors.New("Failed to update Stripe customer.")
)

type StripeError struct {
	Type    string
	Message string
	Code    string
	Param   string
}

func (e StripeError) Error() string {
	return e.Message
}

func New(err error) error {
	stripeErr, ok := err.(*stripe.Error)
	if ok {
		return &StripeError{
			Type:    string(stripeErr.Type),
			Message: stripeErr.Msg,
			Code:    string(stripeErr.Code),
			Param:   string(stripeErr.Param),
		}
	}

	return &StripeError{Type: "unknown", Message: fmt.Sprintf("Stripe error: %v", err)}
}
