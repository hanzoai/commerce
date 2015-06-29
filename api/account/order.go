package account

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

func getOrder(c *gin.Context) {
	usr := middleware.GetUser(c)
	id := c.Params.ByName("orderid")

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	ord := order.New(db)
	if err := ord.GetById(id); err != nil {
		http.Fail(c, 400, "Failed to query order", err)
		return
	}

	if usr.Id() != ord.UserId {
		http.Fail(c, 404, "Order does not exist", errors.New("Order does not exist"))
		return
	}

	http.Render(c, 200, ord)
}

func patchOrder(c *gin.Context) {
	usr := middleware.GetUser(c)
	id := c.Params.ByName("orderid")

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	ord := order.New(db)
	if err := ord.GetById(id); err != nil {
		http.Fail(c, 400, "Failed to query order", err)
		return
	}

	if usr.Id() != ord.UserId {
		http.Fail(c, 404, "Order does not exist", errors.New("Order does not exist"))
		return
	}

	if err := json.Decode(c.Request.Body, ord); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := ord.Put(); err != nil {
		http.Fail(c, 400, "Failed to update order", err)
	}

	http.Render(c, 200, ord)
}
