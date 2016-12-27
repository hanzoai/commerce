package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"

	ds "crowdstart.com/datastore"
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
