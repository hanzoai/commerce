package stripe

import (
	"fmt"
	"net/http"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

/*
Warning
Due to the fact that `CampaignId`s are currently missing in all the orders,
this function assumes that every order is associated with the only campaign.

TODO: Run a migration to set `CampaignId` in all orders.
*/
var SynchronizeCharges = parallel.Task("synchronize-charges", func(db *datastore.Datastore, key datastore.Key, o models.Order, campaign models.Campaign) error {
	for i, charge := range o.Charges {
		client := urlfetch.Client(db.Context)
		url := fmt.Sprintf("https://api.stripe.com/v1/charges/%s", charge.ID)
		chargeReq, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return err
		}
		chargeReq.Header.Add("Authorization", "Basic "+campaign.Stripe.AccessToken)

		res, err := client.Do(chargeReq)
		defer res.Body.Close()
		if err != nil {
			return err
		}

		updatedCharge := new(models.Charge)
		if err := json.Decode(res.Body, updatedCharge); err != nil {
			return err
		}
		o.Charges[i] = *updatedCharge
	}

	for _, charge := range o.Charges {
		if charge.Disputed {
			o.Locked = true
		}
		if charge.Refunded {
			o.Refunded = true
			o.Cancelled = true
		}
	}

	if _, err := db.PutKind("order", key, &o); err != nil {
		return err
	}
	return nil
})

// Create a new stripe customer and assign id to user model.
func createStripeCustomer(ctx appengine.Context, sc *client.API, user *models.User, params *stripe.CustomerParams) error {
	customer, err := sc.Customers.New(params)

	if err != nil {
		log.Warn("Failed to create Stripe customer: %v", err, ctx)
		return err
	}

	// Update user model with stripe customer ID so we can charge for them later
	user.Stripe.CustomerId = customer.ID

	return nil
}

// Update corresponding Stripe customer for this user. If that fails, try to
// create a new customer.
func updateStripeCustomer(ctx appengine.Context, sc *client.API, user *models.User, params *stripe.CustomerParams) error {
	if _, err := sc.Customers.Update(user.Stripe.CustomerId, params); err != nil {
		log.Warn("Failed to update Stripe customer, attempting to create a new Stripe customer: %v", err, ctx)
		return createStripeCustomer(ctx, sc, user, params)
	}
	return nil
}

func Charge(ctx appengine.Context, accessToken string, authorizationToken string, order *models.Order, user *models.User) (*models.Charge, error) {
	c := urlfetch.Client(ctx)
	c.Transport = &urlfetch.Transport{Context: ctx, Deadline: time.Duration(10) * time.Second} // Update deadline to 10 seconds
	backend := stripe.NewInternalBackend(c, "")

	// Stripe advises using client-level methods in a concurrent context
	sc := &client.API{}
	sc.Init(accessToken, backend)

	// Create a charge for us to persist stripe data to
	charge := new(models.Charge)

	// card
	card := &stripe.CardParams{Token: authorizationToken}

	// customer params
	customerParams := &stripe.CustomerParams{
		Desc:  user.Name(),
		Email: user.Email,
		Card:  card,
	}

	if user.Stripe.CustomerId == "" {
		// Create new Stripe customer
		if err := createStripeCustomer(ctx, sc, user, customerParams); err != nil {
			return charge, err
		}
	} else {
		// Update Stripe customer
		if err := updateStripeCustomer(ctx, sc, user, customerParams); err != nil {
			return charge, err
		}
	}

	// Create charge
	log.Debug("Creating charge")
	chargeParams := &stripe.ChargeParams{
		Amount:    order.DecimalTotal(),
		Fee:       order.DecimalFee(),
		Currency:  currency.USD,
		Customer:  user.Stripe.CustomerId,
		Desc:      order.Description(),
		Email:     user.Email,
		Statement: "SKULLY SYSTEMS", // Max 15 characters
	}
	chargeParams.Meta = make(map[string]string)
	chargeParams.Meta["UserId"] = user.Id

	log.Debug("chargeParams: %#v", chargeParams)
	stripeCharge, err := sc.Charges.New(chargeParams)

	// Charges and tokens are recorded regardless of success/failure.
	// It doesn't record whether each charge/token is success or failure.
	if err != nil {
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
