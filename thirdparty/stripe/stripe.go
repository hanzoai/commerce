package stripe

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"appengine"
	"appengine/urlfetch"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

func Charge(ctx appengine.Context, accessToken string, authorizationToken string, order *models.Order) (string, error) {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")
	// Stripe advises using client-level methods
	// in a concurrent context
	sc := &client.API{}
	sc.Init(accessToken, backend)

	log.Debug("Token: %v, Amount: %v", authorizationToken, order.Total, ctx)

	params := &stripe.ChargeParams{
		Amount:   order.DecimalTotal(),
		Fee:      order.DecimalFee(),
		Currency: currency.USD,
		Card:     &stripe.CardParams{Token: authorizationToken},
		Desc:     order.Description(),
	}

	// Force test when email is test@test.com
	if (order.Email == "test@test.com") || (order.Test) {
		log.Debug("Charging in test mode", ctx)
		order.Test = true
		params.Amount = 100
		params.Fee = 2
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
	order.StripeTokens = append(order.StripeTokens, authorizationToken)

	return charge.ID, err
}
