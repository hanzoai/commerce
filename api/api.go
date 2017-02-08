package api

import (
	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/campaign"
	"hanzo.io/models/collection"
	"hanzo.io/models/coupon"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/referral"
	"hanzo.io/models/site"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/token"
	"hanzo.io/models/transaction"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"

	accountApi "hanzo.io/api/account"
	authApi "hanzo.io/api/auth"
	checkoutApi "hanzo.io/api/checkout"
	dataApi "hanzo.io/api/data"
	deployApi "hanzo.io/api/deploy"
	formApi "hanzo.io/api/form"
	orderApi "hanzo.io/api/order"
	organizationApi "hanzo.io/api/organization"
	referrerApi "hanzo.io/api/referrer"
	searchApi "hanzo.io/api/search"
	storeApi "hanzo.io/api/store"
	subscriptionApi "hanzo.io/api/subscription"
	userApi "hanzo.io/api/user"
	paypalApi "hanzo.io/thirdparty/paypal/ipn"
	shipstationApi "hanzo.io/thirdparty/shipstation"
	stripeApi "hanzo.io/thirdparty/stripe/webhook"
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
		api.GET("/ping", router.Ok)
		api.HEAD("/ping", router.Empty)
		api.GET("/humans.txt", router.Humans)
		api.GET("/robots.txt", router.Robots)
	}

	// Use permissive CORS policy for all API routes.
	api.Use(middleware.AccessControl("*"))
	api.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	// Organization APIs, namespaced by organization

	/////////////////////////////////
	// Customer Token/API Key APIs
	/////////////////////////////////
	accountApi.Route(api, tokenRequired)

	///////////////////////////////////
	// API Key and Access Token APIs
	///////////////////////////////////

	// Models with public RESTful API
	rest.New(collection.Collection{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(coupon.Coupon{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(product.Product{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(referral.Referral{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(site.Site{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(submission.Submission{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(transaction.Transaction{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	rest.New(variant.Variant{}).Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	// Checkout APIs (charge, authorize, capture)
	checkoutApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	paymentApi := rest.New(payment.Payment{})
	paymentApi.POST("/:paymentid/refund", checkoutApi.Refund)
	paymentApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	// Hanzo APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/_/"
	campaign.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	organizationApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	token := rest.New(token.Token{})
	token.DefaultNamespace = true
	token.Prefix = "/_/"
	token.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	user := rest.New(user.User{})
	user.DefaultNamespace = true
	user.Prefix = "/_/"
	user.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	deployApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	formApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	orderApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	storeApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	referrerApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	subscriptionApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
	userApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	searchApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	dataApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	// Namespace API
	// namespaceApi.Route(api)

	// Access token API
	// accessTokenApi.Route(api)

	//////////////
	// Webhooks
	//////////////

	// Auth Api
	authApi.Route(api)

	// Shipstation custom store API endpoints
	shipstationApi.Route(api)

	// Stripe webhook
	stripeApi.Route(api)

	// Paypal IPN
	paypalApi.Route(api)
}
