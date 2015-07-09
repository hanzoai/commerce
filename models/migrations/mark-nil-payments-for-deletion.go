package migrations

import (
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"
)

var _ = New("mark-nil-payments-for-deletion",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Account.ChargeId == "" && pay.OrderId == "" && pay.Buyer.UserId == "" {
			pay.Deleted = true
			pay.Put()
			log.Warn("Nil payment found", db.Context)
		}
	})
