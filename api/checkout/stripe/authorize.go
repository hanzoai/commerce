package stripe

import (
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	// Create stripe client
	client := stripe.New(ord.Db.Context, org.StripeToken())

	// Do authorization
	tok, err := client.Authorize(pay)
	if err != nil {
		log.Warn("Failed to authorize payment '%s'", pay.Id())
		log.JSON(pay)
		return err
	}

	// New customer
	if usr.Accounts.Stripe.CustomerId == "" {
		log.Debug("New stripe customer", ord.Db.Context)
		return firstTime(client, tok, usr, ord, pay)
	} else {
		// Existing customer, new card
		log.Debug("Returning stripe customer", ord.Db.Context)
		return returning(client, tok, usr, ord, pay)
	}
}

func updatePaymentFromCard(pay *payment.Payment, card *stripe.Card) {
	pay.Account.CardId = card.ID
	pay.Account.Brand = string(card.Brand)
	pay.Account.LastFour = card.LastFour
	pay.Account.Month = int(card.Month)
	pay.Account.Year = int(card.Year)
	pay.Account.Country = card.Country
	pay.Account.Fingerprint = card.Fingerprint
	pay.Account.Funding = string(card.Funding)
	pay.Account.CVCCheck = string(card.CVCCheck)
}

func updateUserFromPayment(usr *user.User, pay *payment.Payment) {
	usr.Accounts.Stripe.CardId = pay.Account.CardId
	usr.Accounts.Stripe.Brand = string(pay.Account.Brand)
	usr.Accounts.Stripe.LastFour = pay.Account.LastFour
	usr.Accounts.Stripe.Month = int(pay.Account.Month)
	usr.Accounts.Stripe.Year = int(pay.Account.Year)
	usr.Accounts.Stripe.Country = pay.Account.Country
	usr.Accounts.Stripe.Fingerprint = pay.Account.Fingerprint
	usr.Accounts.Stripe.Funding = string(pay.Account.Funding)
	usr.Accounts.Stripe.CVCCheck = string(pay.Account.CVCCheck)
}

func dedupeCards(client *stripe.Client, card *stripe.Card, cust *stripe.Customer, usr *user.User) {
	// Keep track of last four we've seen
	seen := make(map[string]bool)
	seen[card.LastFour] = true

	// Check sources returned on customer for duplicates
	for _, source := range cust.Sources.Values {
		// Skip card we just added
		if card.ID == source.Card.ID {
			continue
		}

		// Delete any dupes
		if _, ok := seen[source.Card.LastFour]; ok {
			if _, err := client.DeleteCard(source.Card.ID, usr); err != nil {
				log.Warn("Unable to delete card '%s': %v", card.ID, err, usr.Db.Context)
			}
		} else {
			seen[source.Card.LastFour] = true
		}

	}
}

func firstTime(client *stripe.Client, tok *stripe.Token, usr *user.User, ord *order.Order, pay *payment.Payment) error {
	// Create Stripe customer, which we will attach to our payment account.
	cust, err := client.NewCustomer(tok.ID, usr)
	if err != nil {
		return err
	}
	pay.Account.CustomerId = cust.ID
	pay.Live = cust.Live

	log.Warn("Stripe New customer: %v", cust, ord.Db.Context)

	// Get default source
	cardId := cust.DefaultSource.ID
	card, err := client.GetCard(cardId, cust.ID)
	if err != nil {
		return err
	}

	// Update account info
	updatePaymentFromCard(pay, card)
	updateUserFromPayment(usr, pay)

	// Save account on user
	usr.Accounts.Stripe = pay.Account.Stripe

	// Create charge and associate with payment.
	_, err = client.NewCharge(cust, pay)
	return err
}

func returning(client *stripe.Client, tok *stripe.Token, usr *user.User, ord *order.Order, pay *payment.Payment) error {
	// Add card to customer
	card, err := client.NewCard(tok.ID, usr)
	if err != nil {
		return err
	}

	// Update account info
	updatePaymentFromCard(pay, card)
	updateUserFromPayment(usr, pay)

	log.Warn("Stripe Returning: %v", pay, ord.Db.Context)

	// Update customer (which will set new card as default)
	cust, err := client.UpdateCustomer(usr)
	if err != nil {
		return err
	}
	pay.Live = cust.Live

	dedupeCards(client, card, cust, usr)

	// Charge using Stripe Customer Id on user
	_, err = client.NewCharge(usr, pay)
	return err
}
