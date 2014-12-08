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
		Amount:   order.DecimalTotal(),
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: token},
		Desc:     order.Description(),
		Fee:      order.DecimalFee(),
	}

	stripeCharge, err := sc.Charges.New(params)

	// Charges and tokens are recorded regardless of success/failure.
	// It doesn't record whether each charge/token is success or failure.
	// It should be possible to query the stripe api for this though.
	charge := models.Charge{
		stripeCharge.ID,
		stripeCharge.Captured,
		stripeCharge.Created,
		stripeCharge.Desc,
		stripeCharge.Email,
		stripeCharge.FailCode,
		stripeCharge.FailMsg,
		stripeCharge.Live,
		stripeCharge.Paid,
		stripeCharge.Refunded,
		stripeCharge.Statement,
		// TODO: Figure out if this is dangerous
		int64(stripeCharge.Amount),
		int64(stripeCharge.AmountRefunded),
	}

	order.Charges = append(order.Charges, charge)
	order.StripeTokens = append(order.StripeTokens, token)

	return charge.ID, err
}
