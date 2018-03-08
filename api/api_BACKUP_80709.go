package api

import (
	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/campaign"
	"hanzo.io/models/collection"
<<<<<<< HEAD
	"hanzo.io/models/coupon"
=======
	"hanzo.io/models/copy"
	"hanzo.io/models/discount"
	"hanzo.io/models/media"
	"hanzo.io/models/note"
>>>>>>> update-go-sdk
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/referral"
	"hanzo.io/models/return"
	"hanzo.io/models/site"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/token"
<<<<<<< HEAD
	"hanzo.io/models/transaction"
=======
	// "hanzo.io/models/transaction"
	"hanzo.io/models/transfer"
>>>>>>> update-go-sdk
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"

	accountApi "hanzo.io/api/account"
<<<<<<< HEAD
	authApi "hanzo.io/api/auth"
	checkoutApi "hanzo.io/api/checkout"
	dataApi "hanzo.io/api/data"
	deployApi "hanzo.io/api/deploy"
	formApi "hanzo.io/api/form"
=======
	affiliateApi "hanzo.io/api/affiliate"
	authApi "hanzo.io/api/auth"
	campaignApi "hanzo.io/api/campaign"
	cartApi "hanzo.io/api/cart"
	cdnApi "hanzo.io/api/cdn"
	checkoutApi "hanzo.io/api/checkout"
	counterApi "hanzo.io/api/counter"
	couponApi "hanzo.io/api/coupon"
	dataApi "hanzo.io/api/data"
	deployApi "hanzo.io/api/deploy"
	formApi "hanzo.io/api/form"
	libraryApi "hanzo.io/api/library"
	marketingApi "hanzo.io/api/marketing"
	namespaceApi "hanzo.io/api/namespace"
>>>>>>> update-go-sdk
	orderApi "hanzo.io/api/order"
	organizationApi "hanzo.io/api/organization"
	referrerApi "hanzo.io/api/referrer"
	searchApi "hanzo.io/api/search"
	storeApi "hanzo.io/api/store"
<<<<<<< HEAD
	subscriptionApi "hanzo.io/api/subscription"
	userApi "hanzo.io/api/user"
	paypalApi "hanzo.io/thirdparty/paypal/ipn"
	reamazeApi "hanzo.io/thirdparty/reamaze"
	shipstationApi "hanzo.io/thirdparty/shipstation"
	stripeApi "hanzo.io/thirdparty/stripe/webhook"
=======
	transactionApi "hanzo.io/api/transaction"
	userApi "hanzo.io/api/user"
	xdApi "hanzo.io/api/xd"

	bitcoinApi "hanzo.io/thirdparty/bitcoin/api"
	ethereumApi "hanzo.io/thirdparty/ethereum/api"
	paypalApi "hanzo.io/thirdparty/paypal/ipn"
	reamazeApi "hanzo.io/thirdparty/reamaze"
	shipstationApi "hanzo.io/thirdparty/shipstation"
	shipwireApi "hanzo.io/thirdparty/shipwire/api"
	stripeApi "hanzo.io/thirdparty/stripe/api"

	dashv2Api "hanzo.io/api/dashv2"

	// Side effect import because of cyclical dependency
	_ "hanzo.io/models/referrer/tasks"
>>>>>>> update-go-sdk
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
<<<<<<< HEAD
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
=======
	rest.New(collection.Collection{}).Route(api, tokenRequired)
	rest.New(copy.Copy{}).Route(api, tokenRequired)
	rest.New(discount.Discount{}).Route(api, tokenRequired)
	rest.New(media.Media{}).Route(api, tokenRequired)
	rest.New(note.Note{}).Route(api, tokenRequired)
	rest.New(product.Product{}).Route(api, tokenRequired)
	rest.New(referral.Referral{}).Route(api, tokenRequired)
	rest.New(return_.Return{}).Route(api, tokenRequired)
	rest.New(site.Site{}).Route(api, tokenRequired)
	rest.New(submission.Submission{}).Route(api, tokenRequired)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired)
	// rest.New(transaction.Transaction{}).Route(api, tokenRequired)
	rest.New(transfer.Transfer{}).Route(api, tokenRequired)
	rest.New(variant.Variant{}).Route(api, tokenRequired)
	rest.New(webhook.Webhook{}).Route(api, tokenRequired)
>>>>>>> update-go-sdk

	paymentApi := rest.New(payment.Payment{})
	paymentApi.POST("/:paymentid/refund", checkoutApi.Refund)
	paymentApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

<<<<<<< HEAD
	// Hanzo APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/_/"
	campaign.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)

	organizationApi.Route(api, tokenRequired, middleware.ApiKeyOrAccessTokenOnly)
=======
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
	transactionApi.Route(api, tokenRequired)
	userApi.Route(api, tokenRequired)

	// Hanzo APIs, using default namespace (internal use only)
	organizationApi.Route(api, tokenRequired)
>>>>>>> update-go-sdk

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

	// OAuth API
	authApi.Route(api)

	// Reamaze custom store API endpoints
	reamazeApi.Route(api)

	// Shipstation custom store API endpoints
	shipstationApi.Route(api)

	// Stripe webhook
	stripeApi.Route(api)

	// Paypal IPN
	paypalApi.Route(api)
<<<<<<< HEAD
=======

	// Data Api
	dataApi.Route(api)

	// XDomain proxy.html
	xdApi.Route(api)

	// Routes from deprecated cdn module
	cdnApi.Route(api)

	// dashv2
	dashv2Api.Route(api)

	// Counter Api (admin only)
	counterApi.Route(api)

	// Library Api
	libraryApi.Route(api)

	// Marketing Api
	marketingApi.Route(api)

	// Bitcoin webhook
	bitcoinApi.Route(api)

	// Ethereum webhook
	ethereumApi.Route(api)
}

func init() {
	api := router.New("api")
	Route(api)
>>>>>>> update-go-sdk
}
