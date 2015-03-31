package payment

import (
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

var orderEndpoint = config.UrlFor("api", "/order2/")

func getOrganizationAndOrder(c *gin.Context) (*organization.Organization, *order.Order) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespace(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	return org, ord
}

func Authorize(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	var err error
	if ord, err = authorize(c, org, ord); err != nil {
		json.Fail(c, 500, "Error during authorize", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	c.JSON(200, ord)
}

func Capture(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	// Fetch order for which we shall capture charges
	id := c.Params.ByName("id")
	if err := ord.Get(id); err != nil {
		json.Fail(c, 500, "Error looking up order", err)
		return
	}

	// Do capture using order we've found
	var err error
	ord, err = capture(c, org, ord)
	if err != nil {
		json.Fail(c, 500, "Error during capture", err)
		return
	}

	c.JSON(200, ord)
}

func Charge(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	// Do authorization
	ord, err := authorize(c, org, ord)
	if err != nil {
		json.Fail(c, 500, "Error during authorize", err)
		return
	}

	// Do capture using order from authorization
	ord, err = capture(c, org, ord)
	if err != nil {
		json.Fail(c, 500, "Error during capture", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	c.JSON(200, ord)
}

func Route(router router.Router) {
	group := router.Group("")
	group.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	group.POST("/charge", publishedRequired, Charge)
	group.POST("/order/:id/charge", publishedRequired, Charge)

	// Two Step Payment API ("Auth & Capture")
	group.POST("/authorize", publishedRequired, Authorize)
	group.POST("/order/:id/authorize", publishedRequired, Authorize)
	group.POST("/order/:id/capture", adminRequired, Capture)
}
