package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"

	. "hanzo.io/models/lineitem"
)

var Order = New("order", func(c *gin.Context) *order.Order {
	db := getNamespaceDb(c)

	u := UserCustomer(c)
	p := Product(c)
	Coupon(c)

	ord := order.New(db)
	ord.UserId = u.Id()
	ord.GetOrCreate("UserId=", ord.UserId)

	ord.Currency = currency.USD
	ord.Items = []LineItem{
		LineItem{
			ProductId: p.Id(),
			Price:     currency.Cents(100),
			Quantity:  20,
		},
	}

	ord.CouponCodes = []string{"SUCH-COUPON", "FREE-DOGE"}
	ord.UpdateAndTally(nil)
	ord.MustPut()

	return ord
})
