package stripe

import "errors"

var (
	FailedToCreateCustomer = errors.New("Failed to create Stripe customer.")
	FailedtoUpdateCustomer = errors.New("Failed to update Stripe customer.")
)
