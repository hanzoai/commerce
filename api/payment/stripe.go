package payment

import (
	"crowdstart.io/models2/user"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/log"
)

func newStripeCustomer(client *stripe.Client, u *user.User, token *stripe.Token) {
	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(token.ID, account.Buyer)
	if err != nil {
		return nil, err
	}
	account.Stripe.CustomerId = customer.ID

	// Save account on user
	user.Accounts[customer.ID] = account

	log.Debug("Stripe customer: %#v", customer)
	log.Debug("Stripe source: %#v", customer.DefaultSource)

	// Get default source
	cardId := customer.DefaultSource.ID
	card, err := client.GetCard(cardId, customer.ID)
	if err != nil {
		return nil, err
	}

	account.Stripe.CardId = cardId
	account.Stripe.Brand = string(card.Brand)
	account.Stripe.LastFour = card.LastFour
	account.Stripe.Expiration.Month = int(card.Month)
	account.Stripe.Expiration.Year = int(card.Year)
	account.Stripe.Country = card.Country
	account.Stripe.Fingerprint = card.Fingerprint
	account.Stripe.Type = string(card.Funding)
	account.Stripe.CVCCheck = string(card.CVCCheck)
}

func oldStripeCustomer(client *stripe.Client, u *user.User, token *stripe.Token) {
}
