package migrations

import (
	"crowdstart.com/models/order"
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
)

var _ = New("reput-orders",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		ord.PutWithoutSideEffects()
	},
)
