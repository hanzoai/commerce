package migrations

import (
	"crowdstart.com/models/order"
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
)

var _ = New("reput-orders-for-kanoa",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		ord.PutWithoutSideEffects()
	},
)
