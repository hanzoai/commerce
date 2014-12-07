package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/template"
)

func DisplayOrders(c *gin.Context) {
	user := auth.GetUser(c)
	db := datastore.New(c)
	var orders []interface{}
	err := db.GetKeyMulti("order", user.OrdersIds, orders)
	if err != nil {
		c.Fail(500, err)
		return
	}

	var cancelled []models.Order
	var shipped []models.Order
	var pending []models.Order

	for _, o := range orders {
		order := o.(models.Order)
		switch {
		case order.Shipped:
			shipped = append(shipped, order)

		case order.Cancelled:
			cancelled = append(cancelled, order)

		default:
			pending = append(pending, order)
		}
	}

	template.Render(c, "orders.html",
		"cancelled", json.Encode(cancelled),
		"shipped", json.Encode(shipped),
		"pending", json.Encode(pending),
	)
}
