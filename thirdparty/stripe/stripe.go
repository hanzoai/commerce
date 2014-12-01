package stripe

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/config"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

func Charge(ctx appengine.Context, token string, order *models.Order) (string, error) {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")

	// Stripe advises using client-level methods
	// in a concurrent context
	sc := &client.API{}
	sc.Init(config.Stripe.APISecret, backend)

	log.Debug("Token: %v, Amount: %v", token, order.Total)

	params := &stripe.ChargeParams{
		Amount:   uint64(order.DecimalTotal()),
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: token},
		Desc:     order.Description(),
	}

	stripeCharge, err := sc.Charges.New(params)

	// Charges and tokens are recorded regardless of success/failure.
	// It doesn't record whether each charge/token is success or failure.
	// It should be possible to query the stripe api for this though.
	charge := models.Charge{
		stripeCharge.ID,
		stripeCharge.Live,
		stripeCharge.Paid,
		stripeCharge.Desc,
		stripeCharge.Email,
		stripeCharge.Amount,
		stripeCharge.FailMsg,
		stripeCharge.Created,
		stripeCharge.Refunded,
		stripeCharge.Captured,
		stripeCharge.FailCode,
		stripeCharge.Statement,
		stripeCharge.AmountRefunded,
	}

	order.Charges = append(order.Charges, charge)
	order.StripeTokens = append(order.StripeTokens, token)

	return charge.ID, err
}
