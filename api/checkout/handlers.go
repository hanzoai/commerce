package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

var orderEndpoint = config.UrlFor("api", "/order/")

func getOrganizationAndOrder(c *gin.Context) (*organization.Organization, *order.Order, error) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	// Get order if an existing order was referenced
	if id := c.Params.ByName("orderid"); id != "" {
		if err := ord.GetById(id); err != nil {
			http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
			return nil, nil, err
		}
	}

	return org, ord, nil
}

func Authorize(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	if _, err = authorize(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	log.JSON(ord)
	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	log.JSON(ord)
	http.Render(c, 200, ord)
}

func Capture(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	if err = capture(c, org, ord); err != nil {
		http.Fail(c, 400, "Error during capture", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Charge(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	// Do authorization
	if _, err = authorize(c, org, ord); err != nil {
		http.Fail(c, 400, "Error during authorize", err)
		return
	}

	// Do capture using order from authorization
	if err = capture(c, org, ord); err != nil {
		http.Fail(c, 400, "Error during capture", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Refund(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	if err := refund(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func Cancel(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	if err := cancel(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func Confirm(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return
	}

	if err := confirm(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func route(router router.Router, prefix string) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := router.Group(prefix)
	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	// Auth and Capture Flow (Two-step Payment)
	api.POST("/authorize", publishedRequired, Authorize)
	api.POST("/authorize/:orderid", publishedRequired, Authorize)
	api.POST("/capture/:orderid", publishedRequired, Capture)

	// Charge Flow (implicit Auth+Capture)
	api.POST("/charge", publishedRequired, Charge)

	// Confirm / Cancel Flow
	api.POST("/confirm/:orderid", publishedRequired, Confirm)
	api.POST("/cancel/:orderid", publishedRequired, Cancel)

	// Deprecated (should use normal authorization flow to initiate)
	api.POST("/paypal", publishedRequired, Authorize)
	api.POST("/paypal/pay", publishedRequired, Authorize)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	route(router, "") // Deprecated
	route(router, "/checkout")
}
