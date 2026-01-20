package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
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
