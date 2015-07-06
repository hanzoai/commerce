package migrations

import (
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"
)

var _ = New("mark-dangling-payments-for-deletion",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		oid := pay.OrderId

		ord := order.New(db)
		if err := ord.GetById(oid); err != nil {
			return
		}

		log.Warn("No Order Found For Payment %v", pay.Id(), db.Context)

		pay.Deleted = true
		if err := pay.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
