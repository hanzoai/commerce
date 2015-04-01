package stripe

import (
	"errors"

	aeds "appengine/datastore"

	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*aeds.Key, []*payment.Payment, error) {
	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	// Get client we can use for API calls
	client := stripe.New(ctx, org.StripeToken())

	payments := make([]*payment.Payment, 0)
	keys, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	if err != nil {
		return nil, nil, nil, err
	}

	num := len(keys)
	log.Warn("payments %v", num)

	// Capture any uncaptured payments
	for i := 0; i < num; i++ {
		p := payments[i]

		if !p.Captured {
			ch, err := client.Capture(p.Account.ChargeId)

			// Charge failed for some reason, bail
			if err != nil {
				return nil, keys, payments, err
			}
			if !ch.Captured {
				return nil, keys, payments, FailedToCaptureCharge
			}

			// Update payment
			p.Captured = true
			p.Status = payment.Paid
			p.Amount = currency.Cents(ch.Amount)
			p.AmountRefunded = currency.Cents(ch.AmountRefunded)
		}
	}

	return ord, keys, payments, nil
}
