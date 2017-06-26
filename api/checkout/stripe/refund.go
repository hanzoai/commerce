package stripe

import (
	"errors"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/emails"
	"hanzo.io/util/log"
)

var NonStripePayment = errors.New("Only refunds for Stripe payments are supported at the moment. This order may contain non-Stripe payments")
var ZeroRefund = errors.New("Refund `amount` cannot be 0")
var NegativeRefund = errors.New("Refund `amount` must be a positive integer")

func Refund(org *organization.Organization, ord *order.Order, refundAmount currency.Cents) error {
	if refundAmount == currency.Cents(0) {
		return ZeroRefund
	}

	if refundAmount < currency.Cents(0) {
		return NegativeRefund
	}

	// Get namespaced context off order
	db := ord.Db
	ctx := db.Context

	log.JSON(ord)
	log.Dump(refundAmount)
	log.Dump(ord.Total)

	if int64(refundAmount) > int64(ord.Total) {
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
			if !p.Test {
				if _, err := client.RefundPayment(p, p.Amount); err != nil {
					return err
				}
			}
			refundRemaining -= p.Amount
		} else if p.Amount > refundRemaining {
			if !p.Test {
				if _, err := client.RefundPayment(p, refundRemaining); err != nil {
					return err
				}
			}
			refundRemaining = 0
		}

		if refundRemaining == 0 {
			break
		}
	}

	log.Info("Refund amount: %v", refundAmount)
	ord.Refunded = ord.Refunded + refundAmount
	ord.Paid = ord.Paid - refundAmount
	usr := user.New(db)
	usr.GetById(ord.UserId)

	// Determine store to use
	storeId := ord.StoreId
	if storeId == "" {
		storeId = org.DefaultStore
	}

	if ord.Total == ord.Refunded {
		emails.SendFullRefundEmail(ctx, org, ord, usr, payments[0])

		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Delete refunded order in mailchimp
		if err := client.DeleteOrder(storeId, ord); err != nil {
			log.Warn("Failed to delete renfuded Mailchimp order: %v", err, ctx)
		}

		ord.PaymentStatus = payment.Refunded
		ord.Status = order.Cancelled
	} else {
		emails.SendPartialRefundEmail(ctx, org, ord, usr, payments[0])
	}

	return ord.Put()
}
