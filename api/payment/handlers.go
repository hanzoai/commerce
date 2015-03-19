package payment

import (
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"github.com/gin-gonic/gin"
)

func getOrganizationAndOrder(c *gin.Context) (*organization.Organization, *order.Order) {
	// Get organization for this user
	org := middleware.GetOrg(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespace(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	return org, ord
}

func Authorize(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	if order, err := authorize(c, org, ord); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, order)
	}
}

func Capture(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	// Fetch order for which we shall capture charges
	id := c.Params.ByName("id")
	if err := ord.Get(id); err != nil {
		json.Fail(c, 500, OrderDoesNotExist.Error(), err)
		return
	}

	// Do capture using order we've found
	if ord, err := capture(c, org, ord); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, ord)
	}
}

func Charge(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)

	// Do authorization
	ord, err := authorize(c, org, ord)
	if err != nil {
		json.Fail(c, 500, err.Error(), err)
		return
	}

	// Do capture using order from authorization
	if ord, err := capture(c, org, ord); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, ord)
	}
}
