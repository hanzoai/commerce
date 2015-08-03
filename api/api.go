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
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/token"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"

	accessTokenApi "crowdstart.com/api/accesstoken"
	accountApi "crowdstart.com/api/account"
	mailinglistApi "crowdstart.com/api/mailinglist"
	namespaceApi "crowdstart.com/api/namespace"
	orderApi "crowdstart.com/api/order"
	paymentApi "crowdstart.com/api/payment"
	searchApi "crowdstart.com/api/search"
	storeApi "crowdstart.com/api/store"
	userApi "crowdstart.com/api/user"

	shipstationApi "crowdstart.com/thirdparty/shipstation"
	stripeApi "crowdstart.com/thirdparty/stripe/webhook"
)

func init() {
	tokenRequired := middleware.TokenRequired()

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
	rest.New(collection.Collection{}).Route(api, tokenRequired)
	rest.New(coupon.Coupon{}).Route(api, tokenRequired)
	rest.New(payment.Payment{}).Route(api, tokenRequired)
	rest.New(product.Product{}).Route(api, tokenRequired)
	rest.New(referral.Referral{}).Route(api, tokenRequired)
	rest.New(referrer.Referrer{}).Route(api, tokenRequired)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired)
	rest.New(variant.Variant{}).Route(api, tokenRequired)
	rest.New(transaction.Transaction{}).Route(api, tokenRequired)

	accountApi.Route(api, tokenRequired)
	mailinglistApi.Route(api, tokenRequired)
	orderApi.Route(api, tokenRequired)
	storeApi.Route(api, tokenRequired)
	userApi.Route(api, tokenRequired)

	// Crowdstart APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/c/"
	campaign.Route(api, tokenRequired)

	organization := rest.New(organization.Organization{})
	organization.DefaultNamespace = true
	organization.Prefix = "/c/"
	organization.Route(api, tokenRequired)

	token := rest.New(token.Token{})
	token.DefaultNamespace = true
	token.Prefix = "/c/"
	token.Route(api, tokenRequired)

	user := rest.New(user.User{})
	user.DefaultNamespace = true
	user.Prefix = "/c/"
	user.Route(api, tokenRequired)

	searchApi.Route(api, tokenRequired)

	// Namespace API
	namespaceApi.Route(api)

	// Access token API
	accessTokenApi.Route(api)

	// Shipstation custom store API endpoints
	shipstationApi.Route(api)

	// Stripe webhook
	stripeApi.Route(api)
}
