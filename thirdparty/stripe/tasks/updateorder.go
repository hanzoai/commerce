package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/util/log"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx appengine.Context, ns string, orderId string, start time.Time) {
	ctx = getNamespace(ctx, ns)
	db := datastore.New(ctx)
	o := order.New(db)

	err := o.RunInTransaction(func() error {
		o.MustGet(orderId)

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
