package stripe

import (
	"appengine"
	"crowdstart.io/models"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"
)

func Authorize(ctx appengine.Context, order models.Order) error {
	client := &client.API{}
	client.Init("access_token", nil)

	params := &stripe.ChargeParams {
		Amount:   1000,
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token:"tok_14dlcYGBoqcjK6A1Th7tPXfJ"},
		Desc:     "Gopher t-shirt",
	}

	ch, err := client.Charges.New(params)
}
