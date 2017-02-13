package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"

	ds "hanzo.io/datastore"
)

var _ = New("save-order-skus",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "stoned")
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
