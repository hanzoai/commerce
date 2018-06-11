package stripe

import (
	"hanzo.io/models/organization"
	"hanzo.io/models/subscription"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/log"
)

// There's a LOT of junk that's gonna be commented out in here for a bit until
// I can get a handle on exactly how boned this file is.
func Subscribe(org *organization.Organization, usr *user.User, sub *subscription.Subscription) error {
	// Create stripe client
	log.Debug("Entering Subscribe")

	client := stripe.New(usr.Db.Context, org.StripeToken())

	// Do authorization
	tok, err := client.AuthorizeSubscription(sub)
	if err != nil {
		return err
	}

	// New customer
	if usr.Accounts.Stripe.CustomerId == "" {
		log.Debug("New stripe customer")
		return firstTime(client, tok, usr, sub)
	}

	// Existing customer, already have card on record
	if usr.Accounts.Stripe.CardMatches(sub.StripeAccount) {
		log.Debug("Returning stripe customer")
		return returning(client, tok, usr, sub)
	}

	// Existing customer, new card
	log.Debug("Returning stripe customer, new card")
	return returningNewCard(client, tok, usr, sub)
}

func UpdateSubscription(org *organization.Organization, sub *subscription.Subscription) error {
	// Create stripe client
	client := stripe.New(sub.Db.Context, org.StripeToken())

	_, err := client.UpdateSubscription(sub)
	return err

	return nil
}

func Unsubscribe(org *organization.Organization, sub *subscription.Subscription) error {
	// Create stripe client

	client := stripe.New(sub.Db.Context, org.StripeToken())

	_, err := client.CancelSubscription(sub)
	return err
}

func firstTime(client *stripe.Client, tok *stripe.Token, u *user.User, sub *subscription.Subscription) error {
	// Create Stripe customer, which we will attach to our payment account.

	cust, err := client.NewCustomer(tok.ID, u)
	if err != nil {
		return err
	}
	sub.StripeAccount.CustomerId = cust.ID
	sub.Live = cust.Live

	log.Debug("Stripe customer: %#v", cust)

	// Get default source
	cardId := cust.DefaultSource.ID
	card, err := client.GetCard(cardId, cust.ID)
	if err != nil {
		return err
	}

	// Update account info
	updatePaymentFromCard(sub, card)

	// Save account on user
	u.Accounts.Stripe = sub.StripeAccount

	// Create charge and associate with payment.
	_, err = client.NewSubscription(tok.ID, cust, sub)
	return err
}

func updatePaymentFromCard(sub *subscription.Subscription, card *stripe.Card) {
	sub.StripeAccount.CardId = card.ID
	sub.StripeAccount.Brand = string(card.Brand)
	sub.StripeAccount.LastFour = card.LastFour
	sub.StripeAccount.Month = int(card.Month)
	sub.StripeAccount.Year = int(card.Year)
	sub.StripeAccount.Country = card.Country
	sub.StripeAccount.Fingerprint = card.Fingerprint
	sub.StripeAccount.Funding = string(card.Funding)
	sub.StripeAccount.CVCCheck = string(card.CVCCheck)
}

func returning(client *stripe.Client, tok *stripe.Token, usr *user.User, sub *subscription.Subscription) error {
	// Update customer details
	cust, err := client.UpdateCustomer(usr)
	if err != nil {
		return err
	}
	sub.Live = cust.Live

	// Update card details using token
	card, err := client.UpdateCard(tok.ID, usr)
	updatePaymentFromCard(sub, card)

	// Charge using customer
	_, err = client.NewSubscription(tok.ID, cust, sub)
	return err
}

func returningNewCard(client *stripe.Client, tok *stripe.Token, usr *user.User, sub *subscription.Subscription) error {
	// Add new card to customer

	card, err := client.AddCard(tok.ID, usr)
	if err != nil {
		return err
	}

	updatePaymentFromCard(sub, card)

	// Charge using customerId on user
	_, err = client.NewSubscription(tok.ID, usr, sub)
	return err
}
