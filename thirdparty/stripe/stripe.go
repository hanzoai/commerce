package stripe

import (
	"appengine"
	"appengine/urlfetch"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func Charge(ctx appengine.Context, accessToken string, authorizationToken string, order *models.Order, user *models.User) (*models.Charge, error) {
	c := urlfetch.Client(ctx)
	c.Transport = &urlfetch.Transport{Context: ctx, Deadline: time.Duration(10) * time.Second} // Update deadline to 10 seconds
	backend := stripe.NewInternalBackend(c, "")

	// Stripe advises using client-level methods in a concurrent context
	sc := &client.API{}
	sc.Init(accessToken, backend)

	// Create a charge for us to persist stripe data to
	charge := new(models.Charge)

	// Create a card
	card := &stripe.CardParams{Token: authorizationToken}

	if user.Stripe.CustomerId == "" {
		// Create new customer
		customerParams := &stripe.CustomerParams{
			Desc:  user.Name(),
			Email: user.Email,
			Card:  card,
		}

		if customer, err := sc.Customers.New(customerParams); err != nil {
			log.Warn("Failed to create Stripe customer: %v", err)
			return charge, err
		} else {
			// Update user with stripe customer ID so we can charge for them later
			user.Stripe.CustomerId = customer.ID
		}
	}

	// Create charge
	chargeParams := &stripe.ChargeParams{
		Amount:    order.DecimalTotal(),
		Fee:       order.DecimalFee(),
		Currency:  currency.USD,
		Customer:  user.Stripe.CustomerId,
		Desc:      order.Description(),
		Email:     order.Email,
		Statement: "SKULLY SYSTEMS", // Max 15 characters
		Card:      card,
	}

	log.Debug("chargeParams: %#v", chargeParams)
	stripeCharge, err := sc.Charges.New(chargeParams)

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
