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

	router := router.New("api")

	// Index
	if appengine.IsDevAppServer() {
		router.GET("/", middleware.ParseToken, rest.ListRoutes())
	} else {
		router.GET("/", func(c *gin.Context) {
			c.String(200, "ok")
		})
	}

	// Use permissive CORS policy for all API routes.
	router.Use(middleware.AccessControl("*"))
	router.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	// Organization APIs, namespaced by organization

	// One Step Payment API
	paymentApi.Route(router)

	// Models with public RESTful API
	rest.New(coupon.Coupon{}).Route(router, adminRequired)
	rest.New(collection.Collection{}).Route(router, adminRequired)
	rest.New(product.Product{}).Route(router, adminRequired)
	rest.New(user.User{}).Route(router, adminRequired)
	rest.New(payment.Payment{}).Route(router, adminRequired)
	rest.New(variant.Variant{}).Route(router, adminRequired)
	rest.New(subscriber.Subscriber{}).Route(router, adminRequired)

	orderApi.Route(router, adminRequired)
	storeApi.Route(router, adminRequired)
	mailinglistApi.Route(router, adminRequired)

	// Crowdstart APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/c/"
	campaign.Route(router, adminRequired)

	organization := rest.New(organization.Organization{})
	organization.DefaultNamespace = true
	organization.Prefix = "/c/"
	organization.Route(router, adminRequired)

	token := rest.New(token.Token{})
	token.DefaultNamespace = true
	token.Prefix = "/c/"
	token.Route(router, adminRequired)

	user := rest.New(user.User{})
	user.DefaultNamespace = true
	user.Prefix = "/c/"
	user.Route(router, adminRequired)

	// Namespace API
	namespaceApi.Route(router)

	// Access token API
	accessTokenApi.Route(router)
}
