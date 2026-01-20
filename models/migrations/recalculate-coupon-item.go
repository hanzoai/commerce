package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/log"

	ds "github.com/hanzoai/commerce/datastore"
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
