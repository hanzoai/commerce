package authorizenet

import (
	"errors"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/thirdparty/authorizenet"
	"hanzo.io/log"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*payment.Payment, error) {
	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	// Get payments for this order
	payments := make([]*payment.Payment, 0)
	if err := payment.Query(db).Ancestor(ord.Key()).GetModels(&payments); err != nil {
		return nil, payments, err
	}

	log.Debug("payments %v", payments)

	// Get client we can use for API calls
	con := org.AuthorizeNetTokens()

	loginId := con.LoginId
	transactionKey := con.TransactionKey
	key := con.Key

	client := authorizenet.New(ctx, loginId, transactionKey, key, false)

	// Capture any uncaptured payments
	for _, p := range payments {

		if !p.Captured {
			p2, err := client.Capture(p)

			// Charge failed for some reason, bail
			if err != nil {
				return nil, payments, err
			}
			if !p2.Captured {
				return nil, payments, FailedToCaptureCharge
			}

			// Update payment
			p2.Captured = true
			// p.Amount = currency.Cents(ord.Amount)
			// p.AmountRefunded = currency.Cents(p2ch.AmountRefunded)
			// p.Account.BalanceTransactionId = ch.Tx.ID
			// p.AmountTransferred = currency.Cents(ch.Tx.Amount)
			// p.CurrencyTransferred = currency.Type(ch.Tx.Currency)
		}
	}

	return ord, payments, nil
}

