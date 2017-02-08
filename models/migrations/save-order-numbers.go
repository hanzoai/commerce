package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"

	ds "hanzo.io/datastore"
)

var _ = New("save-order-numbers",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "stoned")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		db.Put(ord.Key(), ord)
	},
)
