package migrations

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/payment"

	ds "hanzo.io/datastore"
)

var oldestDate = time.Date(2015, time.April, 30, 0, 0, 0, 0, time.UTC)

var _ = New("nuke-old-test-orders",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "cycliq")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		if ord.CreatedAt.Before(oldestDate) {
			ord.Delete()
		}
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.CreatedAt.Before(oldestDate) {
			pay.Delete()
		}
	},
)
