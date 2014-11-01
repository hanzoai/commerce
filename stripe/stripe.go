package stripe

import (
	"appengine"
	"appengine/urlfetch"
	"crowdstart.io/models"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"
)

var backend stripe.InternalBackend

func Charge(ctx appengine.Context, order models.Order) error {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")
	
	sc := &client.API{}
	sc.Init("access_token", backend)

	params := &stripe.ChargeParams{
		Amount:   1000,
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: order.Campaign.StripeToken},
		Desc:     "Gopher t-shirt",
	}
	
	_, err := sc.Charges.New(params)
	return err
}
