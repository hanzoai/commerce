package tasks

import (
	"time"

	"github.com/stripe/stripe-go/charge"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch *stripe.Charge) {
	pay.Status = payment.Unpaid

	// Update status
	if ch.Captured {
		pay.Status = payment.Paid
	}

	if ch.Status == "failed" {
		pay.Status = payment.Cancelled
	}

	if ch.Refunded {
		pay.Status = payment.Refunded
	}

	if ch.FraudDetails != nil {
		if ch.FraudDetails.UserReport == charge.ReportFraudulent ||
			ch.FraudDetails.StripeReport == charge.ReportFraudulent {
			pay.Status = payment.Fraudulent
		}
	}
}

// Synchronize payment using charge
var ChargeSync = delay.Func("stripe-charge-sync", func(ctx appengine.Context, ns string, token string, ch stripe.Charge, start time.Time) {
	ctx = getNamespacedCtx(ctx, ns)

	// Get payment using charge
	pay, err := getPaymentFromCharge(ctx, &ch)
	if err != nil {
		log.Error("Failed to find payment for charge '%s': %v", ch.ID, err, ctx)
		return
	}

	err = pay.RunInTransaction(func() error {
		// Bail out if someone has updated payment since us
		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-payment' task.`, pay.Id(), ch.ID, ctx)
			return nil
		}
	// Update payment status
	UpdatePaymentFromCharge(pay, &ch)
	if err := pay.Put(); err != nil {
		log.Error("Unable to update payment %#v: %v", pay, err, ctx)
		return
	}

	// Check if we need to sync back changes to charge
	payId, _ := ch.Meta["payment"]
	ordId, _ := ch.Meta["order"]
	usrId, _ := ch.Meta["user"]
	if pay.Id() != payId || pay.OrderId != ordId || pay.Buyer.UserId != usrId {
		// Get a stripe client
		client := stripe.New(ctx, token)

		// Update charge with new metadata
		if _, err := client.UpdateCharge(pay); err != nil {
			log.Error("Unable to update charge for payment '%s': %v", pay.Id(), err, ctx)
		}
	}

	// Update order
	updateOrder.Call(ctx, ns, pay.OrderId, start)
})
