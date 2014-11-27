package checkout

import (
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"
)

// Using a global client for now, but it is possible to create a new client
// for each ChargeToken call.
// It is currently initialized in the init function of the module.
var Sc = &client.API{}

// Charges a purchase to the token
// Amount is in cents
// Description should contain a list of products ordered
func ChargeToken(token, description string, amount uint64) (*stripe.Charge, error) {
	params := &stripe.ChargeParams{
		Amount:   amount,
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: token},
		Desc:     description,
	}

	return Sc.Charges.New(params)
}
