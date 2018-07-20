package subscription

import "errors"

var (
	SubscriptionDoesNotExist  = errors.New("Subscription does not exist")
	PlanDoesNotExist          = errors.New("Plan does not exist")
	UserDoesNotExist          = errors.New("User does not exist")
	PaymentCancelled          = errors.New("Subscription was cancelled")
	FailedToDecodeRequestBody = errors.New("Failed to decode request body")
	FailedToCreateCustomer    = errors.New("Failed to create customer")
	FailedToCreateUser        = errors.New("Failed to create user")
	OnlyOneOfUserBuyerAllowed = errors.New("Only one of user buyer allowed")
	CannotChangeUser          = errors.New("Subscription user cannot be changed")
)
