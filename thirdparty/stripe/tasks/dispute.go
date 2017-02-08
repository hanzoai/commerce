package tasks

import (
	"time"

	"appengine"

	"hanzo.io/models/payment"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"
)

// Update payment from dispute
func UpdatePaymentFromDispute(pay *payment.Payment, dispute *stripe.Dispute) {
	switch dispute.Status {
	case stripe.Won:
		pay.Status = payment.Paid
	case stripe.ChargeRefunded:
		pay.Status = payment.Refunded
	default:
		pay.Status = payment.Disputed
	}
}

// Synchronize payment using dispute
var DisputeSync = delay.Func("stripe-update-disputed-payment", func(ctx appengine.Context, ns string, token string, dispute stripe.Dispute, start time.Time) {
	log.Debug("Dispute %s", dispute, ctx)
	ctx = getNamespacedContext(ctx, ns)

	// Get charge from Stripe
	client := stripe.New(ctx, token)
	ch, err := client.GetCharge(dispute.Charge)
	if err != nil {
		log.Error("Unable to get charge '%s' for dispute %v: %v", dispute.ID, dispute, err, ctx)
		return
	}

	// Get payment for associated charge
	pay, ok, err := getPaymentFromCharge(ctx, ch)
	if err != nil {
		log.Error("Failed to query for payment associated with charge '%s': %v", ch.ID, err, ctx)
		return
	}

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

	// Update payment using dispute
	err = pay.RunInTransaction(func() error {
		UpdatePaymentFromDispute(pay, &dispute)
		// Update fees as necessary
		log.Debug("Fees before: %+v", fees, ctx)
		UpdateFeesFromPayment(fees, pay)
		log.Debug("Fees after: %+v", fees, ctx)

		return pay.Put()
	})

	if err != nil {
		log.Error("Failed to update payment '%s' from charge %v: ", pay.Id(), ch, err, ctx)
		return
	}

	// Update charge if necessary
	updateChargeFromPayment(ctx, token, pay, ch)

	// Update order
	updateOrder.Call(ctx, ns, pay.OrderId, pay.AmountRefunded, start)
})
