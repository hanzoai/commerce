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

	if start.Before(pay.UpdatedAt) {
		log.Warn("Payment '%s' previously updated, bailing out", pay.Id(), ctx)
		return
	}

	// Update payment using charge
	err = pay.RunInTransaction(func() error {
		log.Debug("Before UpdatePaymentFromCharge: %+v", pay, ctx)
		UpdatePaymentFromCharge(pay, &ch)
		log.Debug("After UpdatePaymentFromCharge: %+v", pay, ctx)
		return pay.Put()
	})

	if err != nil {
		log.Error("Failed to update payment '%s' from charge %v: ", pay.Id(), ch, err, ctx)
		return
	}

	// Update charge if necessary
	updateChargeFromPayment(ctx, token, pay, &ch)

	// Update order
	updateOrder.Call(ctx, ns, pay.OrderId, start)
})
