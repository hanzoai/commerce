package paypal

import (
	"errors"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
)

var FailedToCaptureCharge = errors.New("Failed to capture charge")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*payment.Payment, error) {
	db := ord.Db

	payments := make([]*payment.Payment, 0)
	_, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	if err != nil {
		return nil, payments, err
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
		}
	}

	return ord, payments, nil
}
