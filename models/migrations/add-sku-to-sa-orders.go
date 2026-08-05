package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/order"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("add-sku-to-sa-orders",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "stoned")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		for i := range ord.Items {
			if ord.Items[i].ProductSlug == "earphone" {
				ord.Items[i].ProductSKU = "686696998137"
			}
		}
		db.Put(ord.Key(), ord)
	},
)
