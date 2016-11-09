package tasks

import (
	"time"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
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

func updateFeesFromPayment(fees []*fee.Fee, pay *payment.Payment) {
	switch pay.Status {
	case payment.Paid:
		pay.Status = payment.Paid
	case payment.Refunded:
		pay.Status = payment.Refunded
	case payment.Disputed:
	default:
		log.Warn("Unhandled payment state")
	}
}

// Synchronize payment using dispute
var DisputeSync = delay.Func("stripe-update-disputed-payment", func(ctx appengine.Context, ns string, token string, dispute stripe.Dispute, start time.Time) {
	log.Warn("DISPUTE SYNC")
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

	db := datastore.New(ctx)
	fees := make([]*fee.Fee, 0)

	if err := fee.Query(db).Filter("PaymentId=", pay.Id()).GetModels(&fees); err != nil {
		log.Error("Failed to query for fees associated with charge '%s': %v", ch.ID, err, ctx)
		return
	}

	// Update fees as necessary
	updateFeesFromPayment(fees, pay)

	if start.Before(pay.UpdatedAt) {
		log.Warn("Payment '%s' previously updated, bailing out", pay.Id(), ctx)
		return
	}

	// Update payment using dispute
	err = pay.RunInTransaction(func() error {
		UpdatePaymentFromDispute(pay, &dispute)
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
