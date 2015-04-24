package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models/order"
	"crowdstart.io/models/types/currency"

	. "crowdstart.io/models/lineitem"
)

var Order = New("order", func(c *gin.Context) *order.Order {
	db := getNamespaceDb(c)

	u := User(c)
	p := Product(c)

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

	return ord
})
