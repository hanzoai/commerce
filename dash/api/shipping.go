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

	ord := order.New(db)
	id := c.Params.ByName("id")
	ord.MustGetById(id)

	// Shipwire will prevent duplicate order creation for identical external
	// IDs, so this is unnecessary in theory...
	if ord.Fulfillment.Type != "" {
		http.Fail(c, 500, "Order already shipped", errors.New("Order already shipped."))
		return
	}

	usr := user.New(db)
	usr.MustGetById(ord.UserId)

	req := ShipRequest{}
	if err := json.Decode(c.Request.Body, &req); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	if res, err := client.CreateOrder(ord, usr, ServiceLevelCode(req.Service)); err != nil {
		http.Fail(c, res.Status, res.Message+res.Error, err)
	} else {
		http.Render(c, res.Status, ord)
	}
}

func ReturnOrderUsingShipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	ord := order.New(db)
	id := c.Params.ByName("id")
	ord.MustGetById(id)

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	res, err := client.CreateReturn(ord)
	if err != nil {
		http.Fail(c, res.Status, res.Message+res.Error, err)
		return
	}

	http.Render(c, res.Status, ord)
}
