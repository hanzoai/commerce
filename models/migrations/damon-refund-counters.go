package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/counter"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("damon-refund-counters",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "damon")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		if err := counter.IncrOrderRefund(db.Context, ord, int(ord.Refunded), ord.UpdatedAt); err != nil {
			log.Error("IncrOrderRefund Error %v", err, db.Context)
		}
	},
)
