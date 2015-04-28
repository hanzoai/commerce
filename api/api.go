package api

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models/campaign"
	"crowdstart.io/models/collection"
	"crowdstart.io/models/coupon"
	"crowdstart.io/models/organization"
	"crowdstart.io/models/payment"
	"crowdstart.io/models/product"
	"crowdstart.io/models/subscriber"
	"crowdstart.io/models/token"
	"crowdstart.io/models/user"
	"crowdstart.io/models/variant"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"

	accessTokenApi "crowdstart.io/api/accessToken"
	mailinglistApi "crowdstart.io/api/mailinglist"
	namespaceApi "crowdstart.io/api/namespace"
	orderApi "crowdstart.io/api/order"
	paymentApi "crowdstart.io/api/payment"
	storeApi "crowdstart.io/api/store"
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
