package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/cart"
	"hanzo.io/models/coupon"

	ds "hanzo.io/datastore"
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
