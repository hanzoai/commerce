package balance

import (
	"errors"

	aeds "appengine/datastore"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/transaction"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*aeds.Key, []*payment.Payment, error) {
	db := ord.Db

	payments := make([]*payment.Payment, 0)
	keys, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	if err != nil {
		return nil, nil, nil, err
	}

	// Capture any uncaptured payments
	for _, p := range payments {

		if !p.Captured {
			// Update payment
			p.Captured = true

			trans := transaction.New(db)
			trans.Amount = p.Amount
			trans.Currency = p.Currency
			trans.Type = transaction.Withdraw
		}
	}

	return ord, keys, payments, nil
}
