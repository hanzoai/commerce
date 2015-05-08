package api

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/campaign"
	"crowdstart.com/models/collection"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"

	accessTokenApi "crowdstart.com/api/accessToken"
	mailinglistApi "crowdstart.com/api/mailinglist"
	namespaceApi "crowdstart.com/api/namespace"
	orderApi "crowdstart.com/api/order"
	paymentApi "crowdstart.com/api/payment"
	storeApi "crowdstart.com/api/store"
)

func init() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.New("api")

	// Index
	if appengine.IsDevAppServer() {
		api.GET("/", middleware.ParseToken, rest.ListRoutes())
	} else {
		api.GET("/", router.Ok)
		api.HEAD("/", router.Empty)
	}

	// Use permissive CORS policy for all API routes.
	api.Use(middleware.AccessControl("*"))
	api.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	// Organization APIs, namespaced by organization

	// One Step Payment API
	paymentApi.Route(api)

	// Models with public RESTful API
	rest.New(coupon.Coupon{}).Route(api, adminRequired)
	rest.New(collection.Collection{}).Route(api, adminRequired)
	rest.New(product.Product{}).Route(api, adminRequired)
	rest.New(user.User{}).Route(api, adminRequired)
	rest.New(payment.Payment{}).Route(api, adminRequired)
	rest.New(variant.Variant{}).Route(api, adminRequired)
	rest.New(subscriber.Subscriber{}).Route(api, adminRequired)

	orderApi.Route(api, adminRequired)
	storeApi.Route(api, adminRequired)
	mailinglistApi.Route(api, adminRequired)

	// Crowdstart APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/c/"
	campaign.Route(api, adminRequired)

	organization := rest.New(organization.Organization{})
	organization.DefaultNamespace = true
	organization.Prefix = "/c/"
	organization.Route(api, adminRequired)

	token := rest.New(token.Token{})
	token.DefaultNamespace = true
	token.Prefix = "/c/"
	token.Route(api, adminRequired)

	user := rest.New(user.User{})
	user.DefaultNamespace = true
	user.Prefix = "/c/"
	user.Route(api, adminRequired)

	// Namespace API
	namespaceApi.Route(api)

	// Access token API
	accessTokenApi.Route(api)
}
