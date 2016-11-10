package tasks

import (
	"time"

	"github.com/stripe/stripe-go/charge"

	"appengine"

	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch *stripe.Charge) {
	pay.Amount = currency.Cents(ch.Amount)
	pay.AmountRefunded = currency.Cents(ch.AmountRefunded)

	pay.Status = payment.Unpaid

	// Update status
	if ch.Captured {
		pay.Status = payment.Paid
	}

	if ch.Status == "failed" {
		pay.Status = payment.Failed
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
	ctx = getNamespacedContext(ctx, ns)

	// Get payment using charge
	pay, ok, err := getPaymentFromCharge(ctx, &ch)
	if err != nil {
		log.Error("Failed to query for payment associated with charge '%s', namespace: '%s': %v\n%#v", ch.ID, err, ch, ctx)
		return
	}

	log.Warn("Payment Id: %v from ChargeId: %v", pay.Id(), ch.ID, ctx)

	if !ok {
		log.Warn("No payment associated with charge '%s'", ch.ID, ctx)
		return
	}

	if start.Before(pay.UpdatedAt) {
		log.Warn("Payment '%s' previously updated, bailing out", pay.Id(), ctx)
		return
	}

	fees, err := pay.GetFees()
	if err != nil {
		log.Error("Failed to query for fees associated with charge '%s': %v", ch.ID, err, ctx)
		return
	}

	// Update payment using charge
	err = pay.RunInTransaction(func() error {
		log.Debug("Payment before: %+v", pay, ctx)
		UpdatePaymentFromCharge(pay, &ch)
		log.Debug("Payment after: %+v", pay, ctx)
		updateFeesFromPayment(fees, pay)

		return pay.Put()
	})

	if err != nil {
		log.Error("Failed to update payment '%s' from charge %v: ", pay.Id(), ch, err, ctx)
		return
	}

	// Update charge if necessary
	updateChargeFromPayment(ctx, token, pay, &ch)

	// Update order
	if pay.OrderId == "" {
		log.Warn("No order associated with payment: %+v", pay, ctx)
		return
	}

	updateOrder.Call(ctx, ns, pay.OrderId, pay.AmountRefunded, start)
})
