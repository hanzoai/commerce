package stripe

import (
	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func Charge(ctx appengine.Context, accessToken string, authorizationToken string, order *models.Order) (*models.Charge, error) {
	backend := stripe.NewInternalBackend(urlfetch.Client(ctx), "")
	// Stripe advises using client-level methods
	// in a concurrent context
	sc := &client.API{}
	sc.Init(accessToken, backend)

	params := &stripe.ChargeParams{
		Amount:    order.DecimalTotal(),
		Fee:       order.DecimalFee(),
		Currency:  currency.USD,
		Card:      &stripe.CardParams{Token: authorizationToken},
		Desc:      order.Description(),
		Email:     order.Email,
		Statement: "SKULLY SYSTEMS", // Max 15 characters
	}

	log.Debug("Params: %#v", params)

	stripeCharge, err := sc.Charges.New(params)

	charge := new(models.Charge)

	// Charges and tokens are recorded regardless of success/failure.
	// It doesn't record whether each charge/token is success or failure.
	if err != nil {
		// &stripe.Error{Type:"card_error", Msg:"Your card was declined.", Code:"card_declined", Param:"", HTTPStatusCode:402}
		stripeErr, ok := err.(*stripe.Error)
		if ok {
			charge.FailCode = json.Encode(stripeErr.Code)
			charge.FailMsg = stripeErr.Msg
			charge.FailType = json.Encode(stripeErr.Type)
		}
	} else {
		charge.ID = stripeCharge.ID
		charge.Captured = stripeCharge.Captured
		charge.Created = stripeCharge.Created
		charge.Desc = stripeCharge.Desc
		charge.Email = stripeCharge.Email
		charge.Live = stripeCharge.Live
		charge.Paid = stripeCharge.Paid
		charge.Refunded = stripeCharge.Refunded
		charge.Statement = stripeCharge.Statement
		charge.Amount = int64(stripeCharge.Amount)
		charge.AmountRefunded = int64(stripeCharge.AmountRefunded)
	}

	order.Charges = append(order.Charges, *charge)
	order.StripeTokens = append(order.StripeTokens, authorizationToken)

	return charge, err
}
