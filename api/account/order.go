package account

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func getOrder(c *zip.Ctx) error {
	usr := middleware.GetUser(c)
	id := c.Param("orderid")

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	ord := order.New(db)
	if err := ord.GetById(id); err != nil {
		return http.Fail(c, 400, "Failed to query order", err)
	}

	if usr.Id() != ord.UserId {
		return http.Fail(c, 404, "Order does not exist", errors.New("Order does not exist"))
	}

	return http.Render(c, 200, ord)
}

func patchOrder(c *zip.Ctx) error {
	usr := middleware.GetUser(c)
	id := c.Param("orderid")

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	ord := order.New(db)
	if err := ord.GetById(id); err != nil {
		return http.Fail(c, 400, "Failed to query order", err)
	}

	if usr.Id() != ord.UserId {
		return http.Fail(c, 404, "Order does not exist", errors.New("Order does not exist"))
	}

	// We only want to extend the shipping address for right now
	// We use a second instance to decode into
	ord2 := order.New(db)

	// Set the address so we overlay
	ord2.ShippingAddress = ord.ShippingAddress

	// Decode into ord2
	if err := json.DecodeBytes(c.Body(), ord2); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	ord.ShippingAddress = ord2.ShippingAddress

	if err := ord.Put(); err != nil {
		return http.Fail(c, 400, "Failed to update order", err)
	}

	return http.Render(c, 200, ord)
}
