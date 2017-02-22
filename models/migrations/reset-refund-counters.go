package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/util/counter"

	ds "hanzo.io/datastore"
)

var _ = New("reset-refund-counters",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "stoned")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		ctx := db.Context
		if ord.Refunded != ord.Total {
			return
		}
		if ord.StoreId != "" {
			if err := counter.IncrementByAll(ctx, "order.refund.count", ord.StoreId, 1, ord.UpdatedAt); err != nil {
				return
			}
		}
		if err := counter.IncrementByAll(ctx, "order.refund.count", "", 1, ord.UpdatedAt); err != nil {
			return
		}
	},
)
