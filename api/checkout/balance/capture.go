package balance

import (
	"errors"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/transaction"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*aeds.Key, []*payment.Payment, error) {
	db := ord.Db

	payments := make([]*payment.Payment, 0)
	keys, err := payment.Query(db).Ancestor(ord.Key()).LoadAll(&payments)
	if err != nil {
		return nil, nil, nil, err
	}

	// Capture any uncaptured payments
	for _, p := range payments {
		if !p.Captured {
			// Update payment
			p.Captured = true
			p.Status = payment.Paid
			p.Model.Db = db
			p.Model.Entity = p
			p.Put()

			trans := transaction.New(db)
			trans.UserId = ord.UserId
			trans.Amount = p.Amount
			trans.Currency = p.Currency
			trans.Type = transaction.Withdraw
			trans.Test = ord.Test
			trans.Put()
		}
	}

	return ord, keys, payments, nil
}
