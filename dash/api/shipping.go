package api

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/shipwire"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "hanzo.io/thirdparty/shipwire/types"
)

type ShipRequest struct {
	Service ServiceLevelCode `json:"service"`
}

func ShipOrderUsingShipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	if o.Fulfillment.Type != "" {
		http.Fail(c, 500, "Order already shipped", errors.New("Order already shipped."))
		return
	}

	u := user.New(db)
	u.MustGetById(o.UserId)

	shipReq := ShipRequest{}
	if err := json.Decode(c.Request.Body, &shipReq); err != nil {
		http.Fail(c, 500, "Failed to decode request body", err)
		return
	}

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	// log.Error("Using Credentials %s, %s", org.Shipwire.Username, org.Shipwire.Password, c)
	if err := client.CreateOrder(o, u, ServiceLevelCode(shipReq.Service)); err != nil {
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
