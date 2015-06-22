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

// Update payment from charge
func UpdatePaymentFromCharge(pay *payment.Payment, ch stripe.Charge) {
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
}

var UpdatePayment = delay.Func("stripe-update-payment", func(ctx appengine.Context, ns string, ch stripe.Charge, start time.Time) {
	ctx = getNamespace(ctx, ns)

	key, err := getPaymentAncestor(ctx, &ch)
	if err != nil {
		log.Panic("Unable to find payment matching charge: %s, %v", ch.ID, err, ctx)
	}

	db := datastore.New(ctx)
	pay := payment.New(db)

	err = pay.RunInTransaction(func() error {
		// Query by ancestor so we can use a transaction
		if ok, err := pay.Query().Ancestor(key).Filter("Account.ChargeId=", ch.ID).First(); !ok {
			return errors.New(fmt.Sprintf("Unable to retrieve payment for charge (%s), ancestor, (%v):", ch.ID, key, err))
		}
		log.Debug("Payment: %v", pay, ctx)

		// Bail out if someone has updated payment since us
		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-payment' task.`, pay.Id(), ch.ID, ctx)
			return nil
		}

		// Actually update payment
		UpdatePaymentFromCharge(pay, ch)
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
