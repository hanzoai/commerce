package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"github.com/gin-gonic/gin"
	sg "github.com/stripe/stripe-go"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
	"crowdstart.com/util/task"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx appengine.Context, orderId string, start time.Time) {
	db := datastore.New(ctx)
	o := order.New(db)
	o.MustGet(orderId)

	err := o.RunInTransaction(func() error {
		if start.Before(o.UpdatedAt) {
			log.Info(`The Order(%s) has already been updated.
					  Stopping 'stripe-update-order' task.`, o.Id(), ctx)
			return nil
		}
		o.UpdatePaymentStatus()
		return o.Put()
	})

	if err != nil {
		log.Panic("Error updating Order(%s) in 'stripe-update-order' %#v", o.Id(), err, ctx)
	}
})

var UpdatePayment = delay.Func("stripe-update-payment", func(c appengine.Context, ch stripe.Charge, start time.Time) {
	db := datastore.New(c)
	pay := payment.New(db)

	err := pay.RunInTransaction(func() error {
		if ok, err := pay.Query().Filter("Account.ChargeId=", ch.ID).First(); !ok {
			log.Panic("Unable to find payment matching charge: %s", ch.ID, err, c)
		}

		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-payment' task.`, pay.Id(), ch.ID, c)
			return nil
		}

		if ch.Captured {
			pay.Status = payment.Paid
		} else if ch.Refunded {
			pay.Status = payment.Refunded
		} else if ch.Paid {
			pay.Status = payment.Paid
		} else {
			pay.Status = payment.Unpaid
		}

		return pay.Put()
	})

	if err != nil {
		log.Panic("Error updating Payment(%s) in 'stripe-update-payment'. %#v", pay.Id(), err, c)
	}

	updateOrder.Call(c, pay.OrderId)
})

var UpdateDisputedPayment = delay.Func("stripe-update-disputed-payment", func(ctx appengine.Context, dispute stripe.Dispute, start time.Time) {
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

	updateOrder.Call(ctx, pay.OrderId)
})

var SyncCharges = task.Func("stripe-sync-charges", func(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)

	// Get organization off query
	query := c.Request.URL.Query()
	orgname := query.Get("organization")

	// Lookup organization
	if err := org.GetById(orgname); err != nil {
		log.Error("Unable to find organization(%s). %#v", orgname, err, c)
		return
	}

	// Get namespaced context
	ctx := org.Namespace(db.Context)

	// Create stripe client
	client := stripe.New(ctx, org.Stripe.AccessToken)

	// Get all stripe charges
	params := &sg.ChargeListParams{}
	i := client.Charges.List(params)
	for i.Next() {
		// Get next charge
		ch := *i.Charge()

		// Update payment, using the namespaced context (i hope)
		start := time.Now()
		UpdatePayment.Call(ctx, ch, start)
	}

	if err := i.Err(); err != nil {
		log.Error("Error while iterating over charges. %#v", err, ctx)
	}
})
