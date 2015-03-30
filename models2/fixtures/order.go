package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/types/currency"
)

func Order(c *gin.Context) *order.Order {
	db := datastore.New(c)

	u := User(c)
	p := Product(c)
	ord := order.New(db)

	ord.Currency = currency.USD
	ord.UserId = u.Id()
	ord.Items = []models.LineItem{
		models.LineItem{
			ProductId: p.Id(),
			Price:     currency.Cents(100),
			Quantity:  20,
		},
	}

	return ord
}
