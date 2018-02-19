package migrations

import (
	"github.com/gin-gonic/gin"

	ds "hanzo.io/datastore"
	"hanzo.io/models/payment"
	"hanzo.io/log"
)

var _ = New("mark-nil-payments-for-deletion",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Account.ChargeId == "" && pay.OrderId == "" && pay.Buyer.UserId == "" {
			pay.Deleted = true
			pay.Put()
			log.Warn("Nil payment found", db.Context)
		}
	})
