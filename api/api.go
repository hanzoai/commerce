package api

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/collection"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/site"
	"crowdstart.com/models/submission"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/token"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/transfer"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"

	accessTokenApi "crowdstart.com/api/accesstoken"
	accountApi "crowdstart.com/api/account"
	affiliateApi "crowdstart.com/api/affiliate"
	campaignApi "crowdstart.com/api/campaign"
	cartApi "crowdstart.com/api/cart"
	checkoutApi "crowdstart.com/api/checkout"
	couponApi "crowdstart.com/api/coupon"
	cronApi "crowdstart.com/api/cron"
	dataApi "crowdstart.com/api/data"
	deployApi "crowdstart.com/api/deploy"
	formApi "crowdstart.com/api/form"
	namespaceApi "crowdstart.com/api/namespace"
	orderApi "crowdstart.com/api/order"
	organizationApi "crowdstart.com/api/organization"
	searchApi "crowdstart.com/api/search"
	storeApi "crowdstart.com/api/store"
	userApi "crowdstart.com/api/user"
	xdApi "crowdstart.com/api/xd"

	paypalApi "crowdstart.com/thirdparty/paypal/ipn"
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
	rest.New(product.Product{}).Route(api, tokenRequired)
	rest.New(referral.Referral{}).Route(api, tokenRequired)
	rest.New(referrer.Referrer{}).Route(api, tokenRequired)
	rest.New(site.Site{}).Route(api, tokenRequired)
	rest.New(submission.Submission{}).Route(api, tokenRequired)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired)
	rest.New(transaction.Transaction{}).Route(api, tokenRequired)
	rest.New(transfer.Transfer{}).Route(api, tokenRequired)
	rest.New(variant.Variant{}).Route(api, tokenRequired)

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
	storeApi.Route(api, tokenRequired)
	userApi.Route(api, tokenRequired)

	// Crowdstart APIs, using default namespace (internal use only)
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

	// Shipstation custom store API endpoints
	shipstationApi.Route(api)

	// Stripe webhook
	stripeApi.Route(api)

	// Paypal IPN
	paypalApi.Route(api)

	// Data Api
	dataApi.Route(api)

	// Routes for cron.yaml tasks
	cronApi.Route(api)

	// XDomain proxy.html
	xdApi.Route(api)
}
