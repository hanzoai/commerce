package tasks

import (
	"errors"
	"fmt"
	"time"

	"appengine"
	aeds "appengine/datastore"
	"appengine/delay"

	"github.com/gin-gonic/gin"
	sg "github.com/stripe/stripe-go"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/hashid"
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

func getAncestor(ctx appengine.Context, ch stripe.Charge) (*aeds.Key, error) {
	// Try to user order id if possible
	if id, ok := ch.Meta["order"]; ok {
		return hashid.DecodeKey(ctx, id)
	}

	// Try to lookup payment
	db := datastore.New(ctx)
	pay := payment.New(db)

	id, ok := ch.Meta["payment"]

	// Try to get by payment id
	var err error
	if ok {
		err = pay.Get(id)
	}

	// Lookup by charge id
	if !ok || err != nil {
		_, err = pay.Query().Filter("Account.ChargeId=", ch.ID).First()
	}

	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to lookup payment by id (%s) or charge id (%s): %v", id, ch.ID, err, ctx))
	}

	return hashid.DecodeKey(ctx, pay.OrderId)
}

var UpdatePayment = delay.Func("stripe-update-payment", func(ctx appengine.Context, ch stripe.Charge, start time.Time) {
	key, err := getAncestor(ctx, ch)
	if err != nil {
		log.Panic("Unable to find payment matching charge: %s", ch.ID, err, ctx)
	}

	db := datastore.New(ctx)
	pay := payment.New(db)

	err = pay.RunInTransaction(func() error {
		// Query by ancestor so we can use a transaction
		if ok, err := pay.Query().Ancestor(key).Filter("Account.ChargeId=", ch.ID).First(); !ok {
			return errors.New(fmt.Sprintf("Unable to retrieve payment for charge (%s), ancestor, (%v):", ch.ID, key, err))
		}

		// Bail out if someone has updated payment since us
		if start.Before(pay.UpdatedAt) {
			log.Info(`The Payment(%s) associated with Charge(%s) has already been updated.
					  Stopping 'stripe-update-payment' task.`, pay.Id(), ch.ID, ctx)
			return nil
		}

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

		// Save updated payment
		return pay.Put()
	})

	// Panic so we restart if something failed
	if err != nil {
		log.Panic("Error updating payment (%s): %v", pay.Id(), err, ctx)
	}

	// Update order
	updateOrder.Call(ctx, pay.OrderId)
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
		ch := stripe.Charge(*i.Charge())

		// Update payment, using the namespaced context (i hope)
		start := time.Now()
		UpdatePayment.Call(ctx, ch, start)
	}

	if err := i.Err(); err != nil {
		log.Error("Error while iterating over charges. %#v", err, ctx)
	}
})
