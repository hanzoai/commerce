package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/order"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("save-order-numbers",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "stoned")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		db.Put(ord.Key(), ord)
	},
)
