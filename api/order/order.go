package order

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/payment"
	"crowdstart.io/util/json"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"

	paymentApi "crowdstart.io/api/payment"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := rest.New(order.Order{})

	api.POST("/:id/capture", adminRequired, rest.NamespacedMiddleware, paymentApi.Capture)
	api.POST("/:id/charge", publishedRequired, paymentApi.Charge)
	api.POST("/:id/authorize", publishedRequired, paymentApi.Authorize)

	api.GET("/:id/payments", adminRequired, rest.NamespacedMiddleware, func(c *gin.Context) {
		id := c.Params.ByName("id")
		db := datastore.New(c)
		ord := order.New(db)

		err := ord.Get(id)
		if err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve order %v: %v", id, err), err)
		}

		payments := make([]*payment.Payment, 0)
		payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
		c.JSON(200, payments)
	})

	api.Create = func(c *gin.Context) {
		db := datastore.New(c)
		ord := order.New(db)

		// Get underlying product/variant entities
		if err := ord.GetItemEntities(); err != nil {
			json.Fail(c, 400, "Failed to get underlying line items", err)
			return
		}

		// Update line items using that information
		ord.UpdateFromEntities()

		// Tally up order again
		ord.Tally()

		if err := json.Decode(c.Request.Body, ord); err != nil {
			json.Fail(c, 400, "Failed decode request body", err)
			return
		}

		if err := ord.Put(); err != nil {
			json.Fail(c, 500, "Failed to create order", err)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+ord.Id())
			api.JSON(c, 201, ord)
		}
	}

	api.Update = func(c *gin.Context) {
		id := c.Params.ByName("id")
		db := datastore.New(c)
		ord := order.New(db)

		// Get Key, and fail if this didn't exist in datastore
		if _, err := ord.KeyExists(id); err != nil {
			json.Fail(c, 404, "No order found with id: "+id, err)
			return
		}

		// Decode response body to create new order
		if err := json.Decode(c.Request.Body, ord); err != nil {
			json.Fail(c, 400, "Failed decode request body", err)
			return
		}

		// Get underlying product/variant entities
		if err := ord.GetItemEntities(); err != nil {
			json.Fail(c, 400, "Failed to get underlying line items", err)
			return
		}

		// Update line items using that information
		ord.UpdateFromEntities()

		// Tally up order again
		ord.Tally()

		// Replace whatever was in the datastore with our new updated order
		if err := ord.Put(); err != nil {
			json.Fail(c, 500, "Failed to update order", err)
		} else {
			api.JSON(c, 200, ord)
		}
	}

	api.Patch = func(c *gin.Context) {
		id := c.Params.ByName("id")
		db := datastore.New(c)
		ord := order.New(db)

		err := ord.Get(id)
		if err != nil {
			json.Fail(c, 404, "No order found with id: "+id, err)
			return
		}

		if err := json.Decode(c.Request.Body, ord); err != nil {
			json.Fail(c, 400, "Failed decode request body", err)
			return
		}

		// Get underlying product/variant entities
		if err = ord.GetItemEntities(); err != nil {
			json.Fail(c, 400, "Failed to get underlying line items", err)
			return
		}

		// Update line items using that information
		ord.UpdateFromEntities()

		// Tally up order again
		ord.Tally()

		if err := ord.Put(); err != nil {
			json.Fail(c, 500, "Failed to update order", err)
		} else {
			api.JSON(c, 200, ord)
		}
	}

	api.Route(router, args...)
}
