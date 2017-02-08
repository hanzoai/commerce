package stripe

import (
	"errors"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/log"
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
	client := stripe.New(ctx, org.StripeToken())

	// Capture any uncaptured payments
	for _, p := range payments {

		if !p.Captured {
			ch, err := client.Capture(p.Account.ChargeId)

			// Charge failed for some reason, bail
			if err != nil {
				return nil, payments, err
			}
			if !ch.Captured {
				return nil, payments, FailedToCaptureCharge
			}

			// Update payment
			p.Captured = true
			p.Status = payment.Paid
			p.Amount = currency.Cents(ch.Amount)
			p.AmountRefunded = currency.Cents(ch.AmountRefunded)
			p.Account.BalanceTransactionId = ch.Tx.ID
			p.AmountTransferred = currency.Cents(ch.Tx.Amount)
			p.CurrencyTransferred = currency.Type(ch.Tx.Currency)
		}
	}

	return ord, payments, nil
}
