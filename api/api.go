package api

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/collection"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/models2/token"
	"crowdstart.io/models2/user"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"

	accessTokenApi "crowdstart.io/api/accessToken"
	mailinglistApi "crowdstart.io/api/mailinglist"
	orderApi "crowdstart.io/api/order"
	paymentApi "crowdstart.io/api/payment"
	storeApi "crowdstart.io/api/store"
)

func init() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	router := router.New("api")

	// Production index
	if !appengine.IsDevAppServer() {
		router.GET("/", func(c *gin.Context) {
			c.String(200, "ok")
		})
	}

	// Use permissive CORS policy for all API routes.
	cors := middleware.AccessControl("*")
	router.Use(cors)
	router.OPTIONS("*wildcard", func(c *gin.Context) {
		cors(c)
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

	// Access token API
	accessTokenApi.Route(router)

	// Development index with debugging routes
	if appengine.IsDevAppServer() {
		router.GET("/", middleware.ParseToken, rest.ListRoutes())
	}
}
