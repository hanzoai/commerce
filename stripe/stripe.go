package stripe

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/config"
	"crowdstart.io/models"
)

func Charge(ctx appengine.Context, token string, order *models.Order) (string, error) {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")

	// Stripe advises using client-level methods
	// in a concurrent context
	sc := &client.API{}
	sc.Init(config.Get().Stripe.APISecret, backend)

	params := &stripe.ChargeParams{
		Amount:   uint64(order.Total),
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: token},
		Desc:     order.Description(),
	}

	charge, err := sc.Charges.New(params)

	// Charges and tokens are recorded regardless of success/failure.
	// It doesn't record whether each charge/token is success or failure.
	// It should be possible to query the stripe api for this though.
	order.Charges = append(order.Charges, *charge)
	order.StripeTokens = append(order.StripeTokens, token)

	return charge.ID, err
}
