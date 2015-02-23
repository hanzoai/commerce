package stripe

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	stripe "github.com/stripe/stripe-go"
	sClient "github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/currency"
	"github.com/stripe/stripe-go/dispute"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func NewApiClient(ctx appengine.Context, accessToken string) *sClient.API {
	c := urlfetch.Client(ctx)
	c.Transport = &urlfetch.Transport{Context: ctx, Deadline: time.Duration(10) * time.Second} // Update deadline to 10 seconds
	backend := stripe.NewInternalBackend(c, "")

	// Stripe advises using client-level methods in a concurrent context
	sc := &sClient.API{}
	sc.Init(accessToken, backend)
	return sc
}

var SynchronizeCharges = parallel.Task("synchronize-charges", func(db *datastore.Datastore, key datastore.Key, o models.Order, campaign models.Campaign) error {
	println("Synchronising")
	log.Info("Synchronising")
	sc := NewApiClient(db.Context, campaign.Stripe.AccessToken)

	description := o.Description()
	for i, charge := range o.Charges {
		updatedCharge, err := sc.Charges.Get(charge.ID, nil)
		if err != nil {
			return err
		}

		if updatedCharge.Desc != description {
			params := &stripe.ChargeParams{Desc: description}
			var err error
			updatedCharge, err = sc.Charges.Update(charge.ID, params)
			if err != nil {
				return err
			}
		}
		o.Charges[i] = models.Charge{
			ID:             updatedCharge.ID,
			Captured:       updatedCharge.Captured,
			Created:        updatedCharge.Created,
			Desc:           updatedCharge.Desc,
			Email:          updatedCharge.Email,
			FailCode:       updatedCharge.FailCode,
			FailMsg:        updatedCharge.FailMsg,
			Live:           updatedCharge.Live,
			Paid:           updatedCharge.Paid,
			Refunded:       updatedCharge.Refunded,
			Statement:      updatedCharge.Statement,
			Amount:         int64(updatedCharge.Amount), // TODO: Check if this is necessary.
			AmountRefunded: int64(updatedCharge.AmountRefunded),
		}

		if updatedCharge.Dispute != nil {
			o.Disputed = true
			if updatedCharge.Dispute.Status != dispute.Won {
				o.Locked = true
			}
		}
	}

	if _, err := db.PutKind("order", key, &o); err != nil {
		return err
	}
	return nil
})

// Create a new stripe customer and assign id to user model.
func createStripeCustomer(ctx appengine.Context, sc *sClient.API, user *models.User, params *stripe.CustomerParams) error {
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
func updateStripeCustomer(ctx appengine.Context, sc *sClient.API, user *models.User, params *stripe.CustomerParams) error {
	if _, err := sc.Customers.Update(user.Stripe.CustomerId, params); err != nil {
		log.Warn("Failed to update Stripe customer, attempting to create a new Stripe customer: %v", err, ctx)
		return createStripeCustomer(ctx, sc, user, params)
	}
	return nil
}

func Charge(ctx appengine.Context, accessToken string, authorizationToken string, order *models.Order, user *models.User) (*models.Charge, error) {
	sc := NewApiClient(ctx, accessToken)

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
