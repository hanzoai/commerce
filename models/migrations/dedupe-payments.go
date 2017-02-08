package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/payment"

	ds "hanzo.io/datastore"
)

var _ = New("dedupe-payments",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		// Bail out if we've been previously deleted
		if pay.Deleted {
			return
		}

		// See if we have a valid order
		ord := order.New(db)
		if err := ord.GetById(pay.OrderId); err != nil {
			// Not a good payment, no matching order
			deletePayment(db.Context, pay)
		}
	},
)
