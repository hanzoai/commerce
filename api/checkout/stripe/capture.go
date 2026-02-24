package stripe

import (
	"errors"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/stripe"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*payment.Payment, error) {
	// Get namespaced context off order
	db := ord.Datastore()
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
			p.Account.BalanceTransactionId = ch.BalanceTransaction.ID
			p.AmountTransferred = currency.Cents(ch.Amount)
			p.CurrencyTransferred = currency.Type(ch.Currency)
		}
	}

	return ord, payments, nil
}
