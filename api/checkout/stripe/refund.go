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

var NonStripePayment = errors.New("Only refunds for Stripe payments are supported at the moment. This order may contain non-Stripe payments")

func Refund(org *organization.Organization, ord *order.Order, refundAmount currency.Cents) error {
	// Get namespaced context off order
	db := ord.Db
	log.Info("Db %#v", db)
	ctx := db.Context
	log.Info("Context %#v", ctx)

	if refundAmount > ord.Total {
		return errors.New("Requested refund amount is greater than the order total")
	}
	if ord.Refunded+refundAmount > ord.Total {
		return errors.New("Previously refunded amounts and requested refund amount exceed the order total")
	}

	payments, err := ord.GetPayments()
	if err != nil {
		return err
	}

	for _, pay := range payments {
		if pay.Type != payment.Stripe {
			return NonStripePayment
		}
	}

	if ord.Paid < refundAmount {
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
	ord.Paid -= refundAmount

	log.Info("Refunded order: %#v", ord)
	err = ord.Put()
	log.Info("Persisted refunded order: %v", err)
	return err
}
