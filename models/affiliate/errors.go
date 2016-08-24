package affiliate

import "fmt"

type StripeAccessTokenNotFound struct {
	UserId     string
	LiveUserId string
	TestUserId string
}

func (e StripeAccessTokenNotFound) Error() string {
	return fmt.Sprintf("Stripe access token not found: User id '%v' doesn't match the Live user id '%v' or the Test user id '%v'",
		e.UserId, e.LiveUserId, e.TestUserId)
}
