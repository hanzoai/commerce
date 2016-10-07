package balance

import (
	"errors"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/log"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*payment.Payment, error) {
	db := ord.Db

	// Get payments for this order
	payments := make([]*payment.Payment, 0)
	if _, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments); err != nil {
		return nil, payments, err
	}

	log.Debug("payments %v", payments)

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

	return ord, payments, nil
}
