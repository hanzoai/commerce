package tasks

import (
	"time"

	"github.com/stripe/stripe-go/charge"

	"appengine"
	"appengine/delay"

	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch *stripe.Charge) {
	// Update status
	if ch.Captured {
		pay.Status = payment.Paid
	} else if ch.Refunded {
		pay.Status = payment.Refunded
	} else if ch.Paid {
		pay.Status = payment.Paid
	} else {
		pay.Status = payment.Unpaid
	}

	if ch.FraudDetails != nil {
		if ch.FraudDetails.UserReport == charge.ReportFraudulent ||
			ch.FraudDetails.StripeReport == charge.ReportFraudulent {
			pay.Status = payment.Fraudulent
		}
	}
}

// Synchronize payment using charge
var ChargeSync = delay.Func("stripe-update-payment", func(ctx appengine.Context, ns string, token string, ch stripe.Charge, start time.Time) {
	ctx = getNamespacedCtx(ctx, ns)

	// Get ancestor (order) using charge
	pay, err := getPaymentFromCharge(ctx, &ch)
	if err != nil {
		log.Panic("Unable to find payment matching charge: %s, %v", ch.ID, err, ctx)
	}

	err = pay.RunInTransaction(func() error {
		// Bail out if someone has updated payment since us
		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-payment' task.`, pay.Id(), ch.ID, ctx)
			return nil
		}

		// Actually update payment
		UpdatePaymentFromCharge(pay, &ch)
		log.Debug("Payment updated to: %v", pay, ctx)

		// Save updated payment
		return pay.Put()
	})

	// Panic so we restart if something failed
	if err != nil {
		log.Panic("Error updating payment (%s): %v", pay.Id(), err, ctx)
	}

	// Update order
	updateOrder.Call(ctx, ns, pay.OrderId, start)
})
