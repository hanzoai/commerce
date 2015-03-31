package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/types/currency"

	. "crowdstart.io/models2/lineitem"
)

func Order(c *gin.Context) *order.Order {
	db := datastore.New(c)

	u := User(c)
	p := Product(c)
	ord := order.New(db)

	ord.Currency = currency.USD
	ord.UserId = u.Id()
	ord.Items = []LineItem{
		LineItem{
			ProductId: p.Id(),
			Price:     currency.Cents(100),
			Quantity:  20,
		},
	}

	return ord
}
