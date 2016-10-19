package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/types/currency"

	. "crowdstart.com/models/lineitem"
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
