package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/store"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("recalculate-coupon-items",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		if len(ord.CouponCodes) > 0 {
			stor := store.New(db)
			if ord.StoreId != "" {
				if err := stor.GetById(ord.StoreId); err != nil {
					log.Error("Could not find store %v", err, db.Context)
					ord.StoreId = ""
					stor = nil
				}
			}

			ord.UpdateAndTally(nil)
			ord.MustPut()
		}
	},
)
