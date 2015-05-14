package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("flag-orders-as-test",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "cycliq")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		for _, payId := range ord.PaymentIds {
			pay := payment.New(db)
			if err := pay.GetById(payId); err != nil {
				log.Error(err, db.Context)
				return
			}
			if pay.Test || !pay.Live {
				ord.Test = true
				if err := ord.Put(); err != nil {
					log.Error(err, db.Context)
				}
				return
			}
		}
	},
)
