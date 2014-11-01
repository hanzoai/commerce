package stripe

import (
	"appengine"
	"appengine/urlfetch"
	"crowdstart.io/models"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"
)

func Charge(ctx appengine.Context, order models.Order) error {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")

	sc := &client.API{}
	sc.Init(order.Campaign.StripeKey, backend) // TODO grab this from datastore

	params := &stripe.ChargeParams{
		Amount:   uint64(order.Total),
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: order.StripeToken},
		Desc:     order.Description(),
	}

	_, err := sc.Charges.New(params)
	return err
}
