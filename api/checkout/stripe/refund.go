package stripe

import (
	"errors"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

func Refund(org *organization.Organization, ord *order.Order, refundAmount currency.Cents) error {
	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	if refundAmount > ord.Total {
		return errors.New("Requested refund amount is greater than the order total")
	}
	if ord.Refunded+refundAmount > ord.Total {
		return errors.New("Previously refunded amounts and requested refund amount exceed the order total")
	}

	payments := make([]*payment.Payment, 0)
	_, err := payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	if err != nil {
		return err
	}

	log.Debug("payments %v", payments)

	var amountPaid currency.Cents = 0
	for _, p := range payments {
		amountPaid += p.Amount
	}
	if amountPaid < refundAmount {
		return errors.New("Refund amount exceeds total payment amount")
	}

	// Get client we can use for API calls
	client := stripe.New(ctx, org.StripeToken())

	refundRemaining := refundAmount
	for _, p := range payments {
		if p.Amount <= refundRemaining {
			if _, err := client.RefundPayment(p, p.Amount); err != nil {
				return err
			}
			refundRemaining -= p.Amount
		} else if p.Amount > refundRemaining {
			if _, err := client.RefundPayment(p, refundRemaining); err != nil {
				return err
			}
			refundRemaining = 0
		}

		if refundRemaining == 0 {
			break
		}
	}

	ord.Refunded += refundAmount
	return ord.Put()
}
