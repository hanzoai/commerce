package stripe

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	// Create stripe client
	client := stripe.New(ord.Db.Context, org.StripeToken())

	// Do authorization
	tok, err := client.Authorize(pay)
	if err != nil {
		return err
	}

	// New customer
	if usr.Accounts.Stripe.CustomerId == "" {
		log.Debug("New stripe customer")
		return firstTime(client, tok, usr, ord, pay)
	}

	// Existing customer, new card
	log.Debug("Returning stripe customer, new card")
	return returning(client, tok, usr, ord, pay)
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

func updatePaymentFromUser(pay *payment.Payment, usr *user.User) {
	acct := usr.Accounts.Stripe
	pay.Account.CardId = acct.CardId
	pay.Account.Brand = acct.Brand
	pay.Account.LastFour = acct.LastFour
	pay.Account.Month = acct.Month
	pay.Account.Year = acct.Year
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
	_, err = client.NewCharge(cust, pay)
	return err
}

func returning(client *stripe.Client, tok *stripe.Token, usr *user.User, ord *order.Order, pay *payment.Payment) error {
	// Add card to customer
	card, err := client.AddCard(tok.ID, usr)
	if err != nil {
		return err
	}
	updatePaymentFromCard(pay, card)

	acct := pay.Account

	// Update customer details (which will set new card as default)
	usr.Accounts.Stripe = acct
	cust, err := client.UpdateCustomer(usr)
	if err != nil {
		return err
	}
	pay.Live = cust.Live

	// Remove any other cards that match from customer
	for _, source := range cust.Sources.Values {
		card := source.Card

		// Skip card we just added
		if card.ID == acct.CardId {
			continue
		}

		// Delete any cards that otherwise match (duplicates, generally)
		if card.LastFour == acct.LastFour {
			if _, err := client.DeleteCard(card.ID, usr); err != nil {
				log.Warn("Unable to delete card '%s': %v", card.ID, err, ord.Db.Context)
			}
		}
	}

	// Charge using Stripe Customer Id on user
	_, err = client.NewCharge(usr, pay)
	return err
}
