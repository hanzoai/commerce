package api

import (
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
	// if o.Fulfillment.Type != "" {
	// 	http.Fail(c, 500, "Order already shipped", errors.New("Order already shipped."))
	// 	return
	// }

	usr := user.New(db)
	usr.MustGetById(ord.UserId)

	req := ShipRequest{}
	if err := json.Decode(c.Request.Body, &req); err != nil {
		http.Fail(c, 500, "Failed to decode request body", err)
		return
	}

	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	// log.Error("Using Credentials %s, %s", org.Shipwire.Username, org.Shipwire.Password, c)
	if res, err := client.CreateOrder(ord, usr, ServiceLevelCode(req.Service)); err != nil {
		http.Fail(c, res.Status, res.Message, err)
	} else {
		http.Render(c, 200, res)
	}
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
