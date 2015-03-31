package order

import (
	"fmt"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/payment"
	"crowdstart.io/util/json"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

var orderApi = rest.New(order.Order{})

func Route(router router.Router, args ...gin.HandlerFunc) {
	orderApi.GET("/:id/payments", func(c *gin.Context) {
		id := c.Params.ByName("id")
		db := datastore.New(c)
		ord := order.New(db)

		err := ord.Get(id)
		if err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve order %v: %v", id, err), err)
		}

		payments := make([]*payment.Payment, 0)
		payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
		c.JSON(200, payments)
	})
	orderApi.Route(router, args...)
}
