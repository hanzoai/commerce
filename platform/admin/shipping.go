package admin

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/shipwire"
	"hanzo.io/util/json/http"
)

func ShipOrderUsingShipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	if o.Fulfillment.Integration != "" {
		http.Fail(c, 500, "Order already shipped", errors.New("Order already shipped."))
		return
	}

	u := user.New(db)
	u.MustGetById(o.UserId)

	service := c.Params.ByName("service")

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	_, err := client.CreateOrder(o, u, shipwire.ServiceLevelCode(service))
	if err != nil {
		http.Fail(c, 400, "Failed to query Shipwire", err)
		return
	}

	http.Render(c, 200, org)
}

func ReturnOrderUsingShipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	_, err := client.CreateReturn(o)
	if err != nil {
		http.Fail(c, 400, "Failed to query Shipwire", err)
		return
	}

	http.Render(c, 200, org)
}
