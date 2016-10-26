package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/cart"
	"crowdstart.com/models/coupon"

	ds "crowdstart.com/datastore"
)

var _ = New("fix-coupons-in-cart",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, car *cart.Cart) {
		car.CouponCodes = []string{}
		car.Coupons = []coupon.Coupon{}
		car.Put()
	},
)
