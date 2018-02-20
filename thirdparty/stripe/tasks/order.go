package tasks

import (
	"context"
	"time"

	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"
)

var updateOrder = delay.Func("stripe-update-order", func(ctx context.Context, ns string, orderId string, refunded currency.Cents, start time.Time) {
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
	}, nil)

	if err != nil {
		log.Error("Failed to update order '%s': %v", orderId, err, ctx)
	}
})
