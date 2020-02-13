package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/util/counter"

	ds "hanzo.io/datastore"
)

var _ = New("damon-refund-counters",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "damon")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		if err := counter.IncrOrderRefund(db.Context, ord, int(ord.Refunded), ord.UpdatedAt); err != nil {
			log.Error("IncrOrderRefund Error %v", err, db.Context)
		}
	},
)
