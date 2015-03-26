package stripe

import (
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/log"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	// Create stripe client
	client := stripe.New(ord.Db.Context, org.Stripe.AccessToken)

	// Do authorization
	tok, err := client.Authorize(pay)
	if err != nil {
		return err
	}

	// New customer
	if usr.Accounts.Stripe.CustomerId == "" {
		return firstTime(client, tok, usr, ord, pay)
	}

	// Existing customer, already have card on record
	if usr.Accounts.Stripe.CardMatches(pay.Account) {
		return returning(client, tok, usr, ord, pay)
	}

	// Existing customer, new card
	return returningNewCard(client, tok, usr, ord, pay)
}

func updatePaymentFromCard(pay *payment.Payment, card *stripe.Card) {
	pay.Account.CardId = card.ID
	pay.Account.Brand = string(card.Brand)
	pay.Account.LastFour = card.LastFour
	pay.Account.Expiration.Month = string(card.Month)
	pay.Account.Expiration.Year = string(card.Year)
	pay.Account.Country = card.Country
	pay.Account.Fingerprint = card.Fingerprint
	pay.Account.Funding = string(card.Funding)
	pay.Account.CVCCheck = string(card.CVCCheck)
}

func updatePaymentFromUser(pay *payment.Payment, usr *user.User) {
	acct := usr.Accounts.Stripe
	pay.Account.CardId = acct.CardId
	pay.Account.Brand = acct.Brand
	pay.Account.LastFour = acct.LastFour
	pay.Account.Expiration.Month = acct.Expiration.Month
	pay.Account.Expiration.Year = acct.Expiration.Year
	pay.Account.Country = acct.Country
	pay.Account.Fingerprint = acct.Fingerprint
	pay.Account.Funding = acct.Funding
	pay.Account.CVCCheck = acct.CVCCheck
}

func firstTime(client *stripe.Client, tok *stripe.Token, u *user.User, ord *order.Order, pay *payment.Payment) error {
	// Create Stripe customer, which we will attach to our payment account.
	cust, err := client.NewCustomer(tok.ID, u)
	if err != nil {
		return err
	}
	pay.Account.CustomerId = cust.ID
	pay.Live = cust.Live

	log.Debug("Stripe customer: %#v", cust)

	// Get default source
	cardId := cust.DefaultSource.ID
	card, err := client.GetCard(cardId, cust.ID)
	if err != nil {
		return err
	}

	// Update account info
	updatePaymentFromCard(pay, card)

	// Save account on user
	u.Accounts.Stripe = pay.Account

	// Create charge and associate with payment.
	charge, err := client.NewCharge(cust, pay)
	if err != nil {
		return err
	}
	pay.ChargeId = charge.ID

	return nil
}

func returning(client *stripe.Client, tok *stripe.Token, usr *user.User, ord *order.Order, pay *payment.Payment) error {
	// Old card, set as source on customer and let's use that to charge
	cust, err := client.UpdateCustomer(tok.ID, usr)
	if err != nil {
		return err
	}
	pay.Live = cust.Live

	updatePaymentFromUser(pay, usr)

	// Charge using customer
	_, err = client.NewCharge(cust, pay)
	return err
}

func returningNewCard(client *stripe.Client, tok *stripe.Token, usr *user.User, ord *order.Order, pay *payment.Payment) error {
	// Add new card to customer
	card, err := client.AddCard(tok.ID, usr)
	if err != nil {
		return err
	}

	updatePaymentFromCard(pay, card)

	// Charge using customerId on user
	_, err = client.NewCharge(usr, pay)
	return err
}
