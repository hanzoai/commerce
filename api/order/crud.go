package order

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func Create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	ord := order.New(db)

	// Decode response body to create new order
	if err := json.Decode(c.Request.Body, ord); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		http.Fail(c, 400, "Invalid or incomplete order", err)
		return
	}

	if err := ord.Create(); err != nil {
		http.Fail(c, 500, "Failed to create order", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+ord.Id())
		http.Render(c, 201, ord)
	}
}

func Update(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	ord := order.New(db)

	id := c.Params.ByName("orderid")

	// Ensure order exists
	if _, _, err := ord.IdExists(id); err != nil {
		http.Fail(c, 404, "No order found with id: "+id, err)
		return
	}

	// Ensure id persists across updates
	ord.SetKey(id)

	// Decode response body to create new order
	if err := json.Decode(c.Request.Body, ord); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		http.Fail(c, 400, "Invalid or incomplete order", err)
		return
	}

	// Replace whatever was in the datastore with our new updated order
	if err := ord.Update(); err != nil {
		http.Fail(c, 500, "Failed to update order", err)
	} else {
		http.Render(c, 200, ord)
	}
}

func Patch(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	ord := order.New(db)

	id := c.Params.ByName("orderid")

	// Ensure order exists
	if err := ord.GetById(id); err != nil {
		http.Fail(c, 404, "No order found with id: "+id, err)
		return
	}

	// Decode response body to create new order
	if err := json.Decode(c.Request.Body, ord); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update order with information from datastore and tally
	if err := ord.UpdateAndTally(nil); err != nil {
		http.Fail(c, 400, "Invalid or incomplete order", err)
		return
	}

	if err := ord.Update(); err != nil {
		http.Fail(c, 500, "Failed to update order", err)
	} else {
		http.Render(c, 200, ord)
	}
}
