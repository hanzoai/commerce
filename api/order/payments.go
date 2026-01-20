package order

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/util/json/http"
)

func Payments(c *gin.Context) {
	id := c.Params.ByName("orderid")
	db := datastore.New(c)
	ord := order.New(db)

	err := ord.GetById(id)
	if err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve order %v: %v", id, err), err)
		return
	}

	payments := make([]*payment.Payment, 0)
	payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	http.Render(c, 200, payments)
}
