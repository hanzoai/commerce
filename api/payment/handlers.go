package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
	"crowdstart.com/util/template"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

var orderEndpoint = config.UrlFor("api", "/order/")

func sendOrderConfirmationEmail(c *gin.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	if !org.Email.Enabled || !org.Email.OrderConfirmation.Enabled || org.Mandrill.APIKey == "" {
		return
	}

	html := template.RenderStringFromString(org.Email.OrderConfirmation.Template,
		"order", ord,
		"orderId", ord.Id(),
		"user", usr)

	ctx := middleware.GetAppEngine(c)
	mandrill.Send.Call(ctx, org.Mandrill.APIKey, usr.Email, usr.Name(), org.Email.OrderConfirmation.Subject, html)
}

func getOrganizationAndOrder(c *gin.Context) (*organization.Organization, *order.Order) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespace(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	// Get order if an existing order was referenced
	if id := c.Params.ByName("orderid"); id != "" {
		if err := ord.Get(id); err != nil {
			return nil, nil
		}
	}

	return org, ord
}

func Authorize(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	if _, _, err := authorize(c, org, ord); err != nil {
		http.Fail(c, 500, "Error during authorize", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Capture(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	// Do capture using order we've found
	var err error
	ord, err = capture(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during capture", err)
		return
	}

	http.Render(c, 200, ord)
}

func Charge(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	// Do authorization
	_, usr, err := authorize(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during authorize", err)
		return
	}

	// Do capture using order from authorization
	ord, err = capture(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during capture", err)
		return
	}

	sendOrderConfirmationEmail(c, org, ord, usr)

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("")
	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Charge Payment API
	api.POST("/charge", publishedRequired, Charge)

	// Auth & Capture Pament API (Two Step Payment)
	api.POST("/authorize", publishedRequired, Authorize)
}
