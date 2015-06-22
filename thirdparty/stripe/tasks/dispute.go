package tasks

import (
	"errors"
	"fmt"
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Update payment from dispute
func UpdatePaymentFromDispute(pay *payment.Payment, dispute *stripe.Dispute) {
	switch dispute.Status {
	case "won":
		pay.Status = payment.Paid
	case "charge_refunded":
		pay.Status = payment.Refunded
	default:
		pay.Status = payment.Disputed
	}
}

// Synchronize payment using dispute
var DisputeSync = delay.Func("stripe-update-disputed-payment", func(ctx appengine.Context, ns string, token string, dispute stripe.Dispute, start time.Time) {
	ctx = getNamespace(ctx, ns)

	// Get charge from Stripe
	chargeId := dispute.Charge
	client := stripe.New(ctx, token)
	ch, err := client.GetCharge(chargeId)
	if err != nil {
		log.Panic("Unable to fetch charge (%s) for dispute (%s): %v", chargeId, dispute, err, ctx)
	}

	// Get ancestor (order) using charge
	key, err := getOrderFromCharge(ctx, ch)
	if err != nil {
		log.Panic("Unable to find payment matching charge: %s, %v", chargeId, err, ctx)
	}

	db := datastore.New(ctx)
	pay := payment.New(db)

	pay.RunInTransaction(func() error {
		// Query by ancestor so we can use a transaction
		if ok, err := pay.Query().Ancestor(key).Filter("Account.ChargeId=", ch.ID).First(); !ok {
			return errors.New(fmt.Sprintf("Unable to retrieve payment for charge (%s), ancestor, (%v):", ch.ID, key, err))
		}
		log.Debug("Payment: %v", pay, ctx)

		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-disputed-payment' task.`, pay.Id(), chargeId, ctx)
			return nil
		}

		// Actually update payment
		UpdatePaymentFromDispute(pay, &dispute)
		log.Debug("Payment updated to: %v", pay, ctx)

		return pay.Put()
	})

	updateOrder.Call(ctx, ns, token, pay.OrderId, start)
})
