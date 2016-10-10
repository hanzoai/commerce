package order

import (
	"fmt"

	"github.com/gin-gonic/gin"

	checkoutApi "crowdstart.com/api/checkout"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(order.Order{})

	api.POST("/:orderid/capture", adminRequired, namespaced, checkoutApi.Capture)

	api.POST("/:orderid/refund", adminRequired, namespaced, checkoutApi.Refund)

	api.POST("/:orderid/charge", publishedRequired, namespaced, checkoutApi.Charge)
	api.POST("/:orderid/authorize", publishedRequired, namespaced, checkoutApi.Authorize)

	api.GET("/:orderid/payments", adminRequired, namespaced, func(c *gin.Context) {
		id := c.Params.ByName("orderid")
		db := datastore.New(c)
		ord := order.New(db)

		err := ord.Get(id)
		if err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to retrieve order %v: %v", id, err), err)
			return
		}

		payments := make([]*payment.Payment, 0)
		payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
		http.Render(c, 200, payments)
	})

	api.Create = func(c *gin.Context) {
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
			api.Render(c, 201, ord)
		}
	}

	api.Update = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))

		id := c.Params.ByName("orderid")
		ord := order.New(db)

		// Get Key, and fail if this didn't exist in datastore
		if _, _, err := ord.IdExists(id); err != nil {
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

		// Replace whatever was in the datastore with our new updated order
		if err := ord.Update(); err != nil {
			http.Fail(c, 500, "Failed to update order", err)
		} else {
			http.Render(c, 200, ord)
		}
	}

	api.Patch = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))

		id := c.Params.ByName("orderid")
		ord := order.New(db)

		err := ord.Get(id)
		if err != nil {
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

	api.Route(router, args...)
}
