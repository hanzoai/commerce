package order

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func Create(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	ord := order.New(db)

	// Decode response body to create new order
	if err := json.DecodeBytes(c.Body(), ord); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		return http.Fail(c, 400, "Invalid or incomplete order", err)
	}

	if err := ord.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create order", err)
	} else {
		c.SetHeader("Location", c.Path()+"/"+ord.Id())
		return http.Render(c, 201, ord)
	}
}

func Update(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	ord := order.New(db)

	id := c.Param("orderid")

	// Ensure order exists
	if _, _, err := ord.IdExists(id); err != nil {
		return http.Fail(c, 404, "No order found with id: "+id, err)
	}

	// Ensure id persists across updates
	ord.SetKey(id)

	// Decode response body to create new order
	if err := json.DecodeBytes(c.Body(), ord); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		return http.Fail(c, 400, "Invalid or incomplete order", err)
	}

	// Replace whatever was in the datastore with our new updated order
	if err := ord.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update order", err)
	} else {
		return http.Render(c, 200, ord)
	}
}

func Patch(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	ord := order.New(db)

	id := c.Param("orderid")

	// Ensure order exists
	if err := ord.GetById(id); err != nil {
		return http.Fail(c, 404, "No order found with id: "+id, err)
	}

	// Decode response body to create new order
	if err := json.DecodeBytes(c.Body(), ord); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		return http.Fail(c, 400, "Invalid or incomplete order", err)
	}

	if err := ord.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update order", err)
	} else {
		return http.Render(c, 200, ord)
	}
}
