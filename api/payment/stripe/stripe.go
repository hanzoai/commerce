package stripe

import (
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/log"
)

func Authorize(org *organization.Organization, ord *order.Order, u *user.User, pay *payment.Payment) error {
	// Create stripe client
	client := stripe.New(ord.Db.Context, org.Stripe.AccessToken)

	// Do authorization
	tok, err := client.Authorize(pay)
	if err != nil {
		return err
	}

	// Handle new, returning customer flows
	if u.Accounts.Stripe.CustomerId == "" {
		return newCustomer(client, tok, u, ord, pay)
	} else {
		return returningCustomer(client, tok, u, ord, pay)
	}
}

func newCustomer(client *stripe.Client, tok *stripe.Token, u *user.User, ord *order.Order, pay *payment.Payment) error {
	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(tok.ID, pay.Buyer)
	if err != nil {
		return err
	}
	pay.Account.CustomerId = customer.ID

	log.Debug("Stripe customer: %#v", customer)
	log.Debug("Stripe source: %#v", customer.DefaultSource)

	// Get default source
	cardId := customer.DefaultSource.ID
	card, err := client.GetCard(cardId, customer.ID)
	if err != nil {
		return err
	}

	// Update account info
	pay.Account.CardId = cardId
	pay.Account.Brand = string(card.Brand)
	pay.Account.LastFour = card.LastFour
	pay.Account.Expiration.Month = string(card.Month)
	pay.Account.Expiration.Year = string(card.Year)
	pay.Account.Country = card.Country
	pay.Account.Fingerprint = card.Fingerprint
	pay.Account.Funding = string(card.Funding)
	pay.Account.CVCCheck = string(card.CVCCheck)

	// Save account on user
	u.Accounts.Stripe = pay.Account

	// Fill with debug information about user's browser
	// payment.Client =

	// Create charge and associate with payment.
	charge, err := client.NewCharge(customer, pay)
	if err != nil {
		return err
	}
	pay.ChargeId = charge.ID

	return nil
}

func returningCustomer(client *stripe.Client, tok *stripe.Token, u *user.User, ord *order.Order, pay *payment.Payment) error {
	// Do authorization
	_, err := client.Authorize(pay)
	if err != nil {
		return err
	}
	return nil
}
