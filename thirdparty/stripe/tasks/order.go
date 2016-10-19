package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx appengine.Context, ns string, orderId string, refunded currency.Cents, start time.Time) {
	ctx = getNamespacedContext(ctx, ns)
	db := datastore.New(ctx)
	ord := order.New(db)

	log.Debug("Updating order '%s'", orderId, ctx)

	if start.Before(ord.UpdatedAt) {
		log.Warn("Order has already been updated %v", ord, ctx)
		return
	}

	err := ord.RunInTransaction(func() error {
		err := ord.GetById(orderId)
		if err != nil {
			return err
		}

		// Update order using latest payment information
		log.Debug("Order before: %+v", ord, ctx)
		ord.UpdatePaymentStatus()
		log.Debug("Order after: %+v", ord, ctx)

		return ord.Put()
	})

	if err != nil {
		log.Error("Failed to update order '%s': %v", orderId, err, ctx)
	}
})
