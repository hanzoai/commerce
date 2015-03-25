package payment

import (
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/log"
)

func newStripeCustomer(client *stripe.Client, ar *AuthReq, u *user.User) error {
	// Do authorization
	token, err := client.Authorize(ar.Source.Card())
	if err != nil {
		return err
	}

	// Create account
	payment := payment.New(ar.Order.Db)
	payment.Buyer = ar.Source.Buyer()
	payment.Status = "unpaid"
	payment.Account.Type = "stripe"
	payment.Amount = ar.Order.Total
	payment.Currency = ar.Order.Currency

	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(token.ID, payment.Buyer)
	if err != nil {
		return err
	}
	payment.Account.CustomerId = customer.ID

	// Save account on user
	u.Accounts.Stripe = payment.Account

	log.Debug("Stripe customer: %#v", customer)
	log.Debug("Stripe source: %#v", customer.DefaultSource)

	// Get default source
	cardId := customer.DefaultSource.ID
	card, err := client.GetCard(cardId, customer.ID)
	if err != nil {
		return err
	}

	payment.Account.CardId = cardId
	payment.Account.Brand = string(card.Brand)
	payment.Account.LastFour = card.LastFour
	payment.Account.Expiration.Month = int(card.Month)
	payment.Account.Expiration.Year = int(card.Year)
	payment.Account.Country = card.Country
	payment.Account.Fingerprint = card.Fingerprint
	payment.Account.Type = string(card.Funding)
	payment.Account.CVCCheck = string(card.CVCCheck)

	// Fill with debug information about user's browser
	// payment.Client =

	// Create charge and associate with payment.
	charge, err := client.NewCharge(customer, payment)
	if err != nil {
		return err
	}
	payment.ChargeId = charge.ID

	// Create payment
	ar.Order.PaymentIds = append(ar.Order.PaymentIds, payment.Id())

	return nil
}

func oldStripeCustomer(client *stripe.Client, u *user.User, token *stripe.Token) {
}
