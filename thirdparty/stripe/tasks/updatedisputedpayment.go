package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

var UpdateDisputedPayment = delay.Func("stripe-update-disputed-payment", func(ctx appengine.Context, ns string, dispute stripe.Dispute, start time.Time) {
	ctx, _ = appengine.Namespace(ctx, ns)
	db := datastore.New(ctx)
	pay := payment.New(db)

	chargeId := dispute.Charge

	pay.RunInTransaction(func() error {
		ok, err := pay.Query().Filter("Account.ChargeId=", chargeId).First()
		if !ok {
			log.Error("Error retrieving Payment associated with disputed Charge(%s). %#v", chargeId, err, ctx)
			return nil
		}

		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-disputed-payment' task.`, pay.Id(), chargeId, ctx)
			return nil
		}

		switch dispute.Status {
		case "won":
			pay.Status = payment.Paid
		case "charge_refunded":
			pay.Status = payment.Refunded
		default:
			pay.Status = payment.Disputed
		}

		return pay.Put()
	})

	updateOrder.Call(ctx, ns, pay.OrderId, start)
})
