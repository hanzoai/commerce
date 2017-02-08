package order

import (
	"hanzo.io/util/counter"
	"hanzo.io/util/event"
)

// Hooks
func (o *Order) AfterCreate() error {
	ctx := o.Context()
	key := o.Kind()
	now := o.CreatedAt

	// Increment overall order totals
	counter.Increment(ctx, key)
	counter.IncrementHour(ctx, key, now)
	counter.IncrementDay(ctx, key, now)
	counter.IncrementMonth(ctx, key, now)

	// Increment product/variant totals
	for _, item := range o.Items {
		key := counter.Key("sales", item.Id())
		counter.Increment(ctx, key)
		counter.IncrementHour(ctx, key, now)
		counter.IncrementDay(ctx, key, now)
		counter.IncrementMonth(ctx, key, now)
	}

	// Increment store totals
	if o.StoreId != "" {
		key = counter.Key(o.StoreId, "orders")
		counter.Increment(ctx, key)
		counter.IncrementHour(ctx, key, now)
		counter.IncrementDay(ctx, key, now)
		counter.IncrementMonth(ctx, key, now)

		for _, item := range o.Items {
			key = counter.Key(item.Id(), o.StoreId)
			counter.Increment(ctx, key)
			counter.IncrementHour(ctx, key, now)
			counter.IncrementDay(ctx, key, now)
			counter.IncrementMonth(ctx, key, now)
		}
	}

	return event.Emit(ctx, o.Namespace(), "order.created", o)
}

func (o *Order) AfterUpdate(previous *Order) error {
	return event.Emit(o.Context(), o.Namespace(), "order.updated", o)
}

func (o *Order) AfterDelete() error {
	return event.Emit(o.Context(), o.Namespace(), "order.deleted", o)
}
