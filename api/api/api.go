package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/demo/disclosure"
	"github.com/hanzoai/commerce/demo/tokentransaction"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/collection"
	"github.com/hanzoai/commerce/models/discount"
	"github.com/hanzoai/commerce/models/movie"
	"github.com/hanzoai/commerce/models/note"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/return"
	"github.com/hanzoai/commerce/models/saleschannel"
	"github.com/hanzoai/commerce/models/site"
	"github.com/hanzoai/commerce/models/stocklocation"
	"github.com/hanzoai/commerce/models/submission"
	"github.com/hanzoai/commerce/models/subscriber"
	"github.com/hanzoai/commerce/models/token"
	// "github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/models/wallet"
	"github.com/hanzoai/commerce/models/watchlist"
	"github.com/hanzoai/commerce/models/webhook"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"

	accessTokenApi "github.com/hanzoai/commerce/api/accesstoken"
	accountApi "github.com/hanzoai/commerce/api/account"
	affiliateApi "github.com/hanzoai/commerce/api/affiliate"
	billingApi "github.com/hanzoai/commerce/api/billing"
	authApi "github.com/hanzoai/commerce/api/auth"
	cartApi "github.com/hanzoai/commerce/api/cart"
	cdnApi "github.com/hanzoai/commerce/api/cdn"
	checkoutApi "github.com/hanzoai/commerce/api/checkout"
	counterApi "github.com/hanzoai/commerce/api/counter"
	couponApi "github.com/hanzoai/commerce/api/coupon"
	dataApi "github.com/hanzoai/commerce/api/data"
	deployApi "github.com/hanzoai/commerce/api/deploy"
	formApi "github.com/hanzoai/commerce/api/form"
	libraryApi "github.com/hanzoai/commerce/api/library"
	namespaceApi "github.com/hanzoai/commerce/api/namespace"
	orderApi "github.com/hanzoai/commerce/api/order"
	organizationApi "github.com/hanzoai/commerce/api/organization"
	referrerApi "github.com/hanzoai/commerce/api/referrer"
	regionApi "github.com/hanzoai/commerce/api/region"
	reviewApi "github.com/hanzoai/commerce/api/review"
	searchApi "github.com/hanzoai/commerce/api/search"
	storeApi "github.com/hanzoai/commerce/api/store"
	subscriptionApi "github.com/hanzoai/commerce/api/subscription"
	transactionApi "github.com/hanzoai/commerce/api/transaction"
	inventoryApi "github.com/hanzoai/commerce/api/inventory"
	pricingApi "github.com/hanzoai/commerce/api/pricing"
	promotionApi "github.com/hanzoai/commerce/api/promotion"
	userApi "github.com/hanzoai/commerce/api/user"
	xdApi "github.com/hanzoai/commerce/api/xd"

	bitcoinApi "github.com/hanzoai/commerce/thirdparty/bitcoin/api"
	ethereumApi "github.com/hanzoai/commerce/thirdparty/ethereum/api"
	mercuryApi "github.com/hanzoai/commerce/thirdparty/mercury/api"
	paypalApi "github.com/hanzoai/commerce/thirdparty/paypal/ipn"
	reamazeApi "github.com/hanzoai/commerce/thirdparty/reamaze"
	shipstationApi "github.com/hanzoai/commerce/thirdparty/shipstation"
	shipwireApi "github.com/hanzoai/commerce/thirdparty/shipwire/api"
	stripeApi "github.com/hanzoai/commerce/thirdparty/stripe/api"

	dashv2Api "github.com/hanzoai/commerce/api/dashv2"

	// Side effect import because of cyclical dependency
	_ "github.com/hanzoai/commerce/models/referrer/tasks"
)

func Route(api router.Router) {
	tokenRequired := middleware.TokenRequired()
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Health check — always available regardless of mode
	api.GET("/ping", router.Ok)
	api.HEAD("/ping", router.Empty)

	// Index
	if config.IsDevelopment {
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

	// Setup routes for delay funcs
	api.POST(delay.Path, func(c *gin.Context) {
		ctx := c.Request.Context()
		delay.RunFunc(ctx, c.Writer, c.Request)
	})

	// Checkout APIs (charge, authorize, capture)
	checkoutApi.Route(api)

	subscriptionApi.Route(api)

	// Models with public RESTful API
	rest.New(collection.Collection{}).Route(api, tokenRequired)
	rest.New(discount.Discount{}).Route(api, tokenRequired)
	rest.New(movie.Movie{}).Route(api, tokenRequired)
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
	rest.New(wallet.Wallet{}).Route(api, adminRequired)
	rest.New(watchlist.Watchlist{}).Route(api, tokenRequired)
	rest.New(webhook.Webhook{}).Route(api, adminRequired)

	rest.New(saleschannel.SalesChannel{}).Route(api, tokenRequired)
	rest.New(stocklocation.StockLocation{}).Route(api, tokenRequired)

	rest.New(disclosure.Disclosure{}).Route(api, tokenRequired)
	rest.New(tokentransaction.Transaction{}).Route(api, tokenRequired)

	paymentApi := rest.New(payment.Payment{})
	paymentApi.POST("/:paymentid/refund", checkoutApi.Refund)
	paymentApi.Route(api, tokenRequired)

	accountApi.Route(api, tokenRequired)
	affiliateApi.Route(api, tokenRequired)
	billingApi.Route(api, tokenRequired)
	cartApi.Route(api, tokenRequired)
	couponApi.Route(api, tokenRequired)
	deployApi.Route(api, tokenRequired)
	formApi.Route(api, tokenRequired)
	inventoryApi.Route(api, tokenRequired)
	orderApi.Route(api, tokenRequired)
	referrerApi.Route(api, tokenRequired)
	regionApi.Route(api, tokenRequired)
	reviewApi.Route(api, tokenRequired)
	storeApi.Route(api, tokenRequired)
	transactionApi.Route(api, tokenRequired)
	userApi.Route(api, tokenRequired)
	pricingApi.Route(api, tokenRequired)
	promotionApi.Route(api, tokenRequired)

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

	// OAuth API
	authApi.Route(api)

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
	counterApi.Route(api)

	// Library Api
	libraryApi.Route(api)

	// Marketing routes moved to github.com/hanzoai/marketing

	// Bitcoin webhook
	bitcoinApi.Route(api)

	// Ethereum webhook
	ethereumApi.Route(api)

	// Mercury bank webhook
	mercuryApi.Route(api)
}
