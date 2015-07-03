package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/util/log"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx appengine.Context, ns string, token string, orderId string, start time.Time) {
	ctx = getNamespace(ctx, ns)
	db := datastore.New(ctx)
	ord := order.New(db)

	log.Debug("Updating order (%s)", orderId, ctx)

	err := ord.RunInTransaction(func() error {
		err := ord.Get(orderId)
		if err != nil {
			log.Error("Failed to get order: %v", err, ctx)
			return nil
		}

		if start.Before(ord.UpdatedAt) {
			log.Warn("Order has already been updated %v", ord, ctx)
			return nil
		}

		// Update order using latest payment information
		ord.UpdatePaymentStatus()

		return ord.Put()
	})

	if err != nil {
		log.Panic("Update order transaction failed to get order (%s): %v", orderId, err, ctx)
	}
})
