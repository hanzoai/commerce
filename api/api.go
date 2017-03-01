package api

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/collection"
	"hanzo.io/models/discount"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/referral"
	"hanzo.io/models/return"
	"hanzo.io/models/site"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/token"
	"hanzo.io/models/transaction"
	"hanzo.io/models/transfer"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	"hanzo.io/models/webhook"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"

	accessTokenApi "hanzo.io/api/accesstoken"
	accountApi "hanzo.io/api/account"
	affiliateApi "hanzo.io/api/affiliate"
	campaignApi "hanzo.io/api/campaign"
	cartApi "hanzo.io/api/cart"
	cdnApi "hanzo.io/api/cdn"
	checkoutApi "hanzo.io/api/checkout"
	counterApi "hanzo.io/api/counter"
	couponApi "hanzo.io/api/coupon"
	dataApi "hanzo.io/api/data"
	deployApi "hanzo.io/api/deploy"
	formApi "hanzo.io/api/form"
	namespaceApi "hanzo.io/api/namespace"
	noteApi "hanzo.io/api/note"
	orderApi "hanzo.io/api/order"
	organizationApi "hanzo.io/api/organization"
	referrerApi "hanzo.io/api/referrer"
	reviewApi "hanzo.io/api/review"
	searchApi "hanzo.io/api/search"
	storeApi "hanzo.io/api/store"
	userApi "hanzo.io/api/user"
	xdApi "hanzo.io/api/xd"

	paypalApi "hanzo.io/thirdparty/paypal/ipn"
	reamazeApi "hanzo.io/thirdparty/reamaze"
	shipstationApi "hanzo.io/thirdparty/shipstation"
	shipwireApi "hanzo.io/thirdparty/shipwire/api"
	stripeApi "hanzo.io/thirdparty/stripe/api"

	dashv2Api "hanzo.io/api/dashv2"

	// Side effect import because of cyclical dependency
	_ "hanzo.io/models/referrer/tasks"
)

func Route(api router.Router) {
	tokenRequired := middleware.TokenRequired()
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Index
	if appengine.IsDevAppServer() {
		api.GET("/", middleware.ParseToken, rest.ListRoutes())
	} else {
		api.GET("/", router.Ok)
		api.HEAD("/", router.Empty)

		api.GET("/ping", router.Ok)
		api.HEAD("/ping", router.Empty)
	}

	// Use permissive CORS policy for all API routes.
	api.Use(middleware.AccessControl("*"))
	api.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	// Organization APIs, namespaced by organization

	// Checkout APIs (charge, authorize, capture)
	checkoutApi.Route(api)

	// Models with public RESTful API
	rest.New(collection.Collection{}).Route(api, tokenRequired)
	rest.New(discount.Discount{}).Route(api, tokenRequired)
	rest.New(product.Product{}).Route(api, tokenRequired)
	rest.New(referral.Referral{}).Route(api, tokenRequired)
	rest.New(site.Site{}).Route(api, tokenRequired)
	rest.New(submission.Submission{}).Route(api, tokenRequired)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired)
	rest.New(transaction.Transaction{}).Route(api, tokenRequired)
	rest.New(transfer.Transfer{}).Route(api, tokenRequired)
	rest.New(variant.Variant{}).Route(api, tokenRequired)
	rest.New(return_.Return{}).Route(api, tokenRequired)
	rest.New(webhook.Webhook{}).Route(api, tokenRequired)

	paymentApi := rest.New(payment.Payment{})
	paymentApi.POST("/:paymentid/refund", checkoutApi.Refund)
	paymentApi.Route(api, tokenRequired)

	accountApi.Route(api, tokenRequired)
	affiliateApi.Route(api, tokenRequired)
	campaignApi.Route(api, tokenRequired)
	cartApi.Route(api, tokenRequired)
	couponApi.Route(api, tokenRequired)
	deployApi.Route(api, tokenRequired)
	formApi.Route(api, tokenRequired)
	orderApi.Route(api, tokenRequired)
	referrerApi.Route(api, tokenRequired)
	reviewApi.Route(api, tokenRequired)
	storeApi.Route(api, tokenRequired)
	userApi.Route(api, tokenRequired)

	// Hanzo APIs, using default namespace (internal use only)
	organizationApi.Route(api, tokenRequired)

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

	// Reamaze custom store API endpoints
	reamazeApi.Route(api)

	// Shipstation custom store API endpoints
	shipstationApi.Route(api)

	// Shipwire custom store API endpoints
	shipwireApi.Route(api)

	// Stripe callback, webhook
	stripeApi.Route(api)

	// Paypal IPN
	paypalApi.Route(api)

	// Data Api
	dataApi.Route(api)

	// XDomain proxy.html
	xdApi.Route(api)

	// Routes from deprecated cdn module
	cdnApi.Route(api)

	// dashv2
	dashv2Api.Route(api)

	// Counter Api (admin only)
	counterApi.Route(api, adminRequired)

	// Note Api (admin only)
	noteApi.Route(api, adminRequired)
}

func init() {
	api := router.New("api")
	Route(api)
}
