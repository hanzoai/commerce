package api

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/resources"
	"github.com/hanzoai/commerce/billing/paywall"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/token"

	// "github.com/hanzoai/commerce/models/transaction"

	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"

	accessTokenApi "github.com/hanzoai/commerce/api/accesstoken"
	accountApi "github.com/hanzoai/commerce/api/account"
	affiliateApi "github.com/hanzoai/commerce/api/affiliate"
	apikeyApi "github.com/hanzoai/commerce/api/apikey"
	authApi "github.com/hanzoai/commerce/api/auth"
	b2bApi "github.com/hanzoai/commerce/api/b2b"
	billingApi "github.com/hanzoai/commerce/api/billing"
	cartApi "github.com/hanzoai/commerce/api/cart"
	catalogApi "github.com/hanzoai/commerce/api/catalog"
	cdnApi "github.com/hanzoai/commerce/api/cdn"
	checkoutApi "github.com/hanzoai/commerce/api/checkout"
	claimApi "github.com/hanzoai/commerce/api/claim"
	costsApi "github.com/hanzoai/commerce/api/costs"
	counterApi "github.com/hanzoai/commerce/api/counter"
	couponApi "github.com/hanzoai/commerce/api/coupon"
	currencyApi "github.com/hanzoai/commerce/api/currency"
	customergroupApi "github.com/hanzoai/commerce/api/customergroup"
	dataApi "github.com/hanzoai/commerce/api/data"
	deployApi "github.com/hanzoai/commerce/api/deploy"
	draftorderApi "github.com/hanzoai/commerce/api/draftorder"
	exchangeApi "github.com/hanzoai/commerce/api/exchange"
	formApi "github.com/hanzoai/commerce/api/form"
	fulfillmentApi "github.com/hanzoai/commerce/api/fulfillment"
	giftcardApi "github.com/hanzoai/commerce/api/giftcard"
	inventoryApi "github.com/hanzoai/commerce/api/inventory"
	inviteApi "github.com/hanzoai/commerce/api/invite"
	libraryApi "github.com/hanzoai/commerce/api/library"
	metricsApi "github.com/hanzoai/commerce/api/metrics"
	namespaceApi "github.com/hanzoai/commerce/api/namespace"
	notificationApi "github.com/hanzoai/commerce/api/notification"
	orderApi "github.com/hanzoai/commerce/api/order"
	organizationApi "github.com/hanzoai/commerce/api/organization"
	payablesApi "github.com/hanzoai/commerce/api/payables"
	planApi "github.com/hanzoai/commerce/api/plan"
	pricingApi "github.com/hanzoai/commerce/api/pricing"
	producttaxonomyApi "github.com/hanzoai/commerce/api/producttaxonomy"
	promoApi "github.com/hanzoai/commerce/api/promo"
	promotionApi "github.com/hanzoai/commerce/api/promotion"
	referralApi "github.com/hanzoai/commerce/api/referral"
	regionApi "github.com/hanzoai/commerce/api/region"
	reviewApi "github.com/hanzoai/commerce/api/review"
	searchApi "github.com/hanzoai/commerce/api/search"
	storeApi "github.com/hanzoai/commerce/api/store"
	subscriptionApi "github.com/hanzoai/commerce/api/subscription"
	taxApi "github.com/hanzoai/commerce/api/tax"
	transactionApi "github.com/hanzoai/commerce/api/transaction"
	uploadApi "github.com/hanzoai/commerce/api/upload"
	userApi "github.com/hanzoai/commerce/api/user"
	xdApi "github.com/hanzoai/commerce/api/xd"

	dashv2Api "github.com/hanzoai/commerce/api/dashv2"
	bitcoinApi "github.com/hanzoai/commerce/thirdparty/bitcoin/api"
	mercuryApi "github.com/hanzoai/commerce/thirdparty/mercury/api"
	paypalApi "github.com/hanzoai/commerce/thirdparty/paypal/ipn"
	reamazeApi "github.com/hanzoai/commerce/thirdparty/reamaze"
	shipstationApi "github.com/hanzoai/commerce/thirdparty/shipstation"
	shipwireApi "github.com/hanzoai/commerce/thirdparty/shipwire/api"

	// Side effect import because of cyclical dependency
	_ "github.com/hanzoai/commerce/models/referrer/tasks"
)

func Route(api zip.Router) {
	tokenRequired := middleware.TokenRequired()
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Subscription paywall — the org-level admin gate ($20/mo pro plan OR live
	// trial OR redeemed invite). Mounted as a sibling to tokenRequired, AFTER it
	// so the org is resolved, on the merchant admin surface. It supersedes the
	// store-bound requireAccess (a strict superset: same subscription
	// check across both money stores, plus trial-credit and invite paths).
	// billing/subscribe/trial/invite/catalog routes are NOT behind it — those are
	// how an org acquires access, or are public reads.
	requireAccess := paywall.Require

	// Index
	if config.IsDevelopment {
		api.Get("/", middleware.ParseToken, rest.ListRoutes())
	} else {
		api.Get("/", router.Ok)
		api.Head("/", router.Empty)
	}

	// Use permissive CORS policy for all API routes.
	api.Use(middleware.AccessControl("*"))
	api.Options("*wildcard", func(c *zip.Ctx) error {
		return c.Next()
	})

	// NOTE: there is no HTTP dispatch route for delayed funcs. A delayed func is
	// a typed Go value invoked in process (util/task: `v.Call(ctx, args...)`,
	// with delay.Function verifying AssignableTo at enqueue). The old
	// POST /_/queue/delay route decoded a gob body, looked the function up BY
	// NAME and reflect-called it with caller-supplied arguments — unauthenticated,
	// untyped, and a second way to do what the typed path already does.

	// Checkout APIs (charge, authorize, capture)
	checkoutApi.Route(api)

	subscriptionApi.Route(api)

	// The merchant resources. ONE table, in its own leaf package so the cloud
	// plugin can mount the same set at /v1/commerce/<kind> without importing
	// this package's checkout, subscription and thirdparty tree.
	resources.Route(api, tokenRequired, adminRequired, requireAccess, publishProductEvents)

	paymentApi := rest.New(payment.Payment{})
	paymentApi.POST("/:paymentid/refund", checkoutApi.Refund)
	paymentApi.Route(api, tokenRequired)

	accountApi.Route(api, tokenRequired)
	billingApi.Route(api, tokenRequired)
	costsApi.Route(api, tokenRequired)
	metricsApi.Route(api, tokenRequired) // SaaS ops god-view (revenue/subs/usage/customers) — global-admin gated inside

	cartApi.Route(api, tokenRequired)
	couponApi.Route(api, tokenRequired)
	// Subscription paywall invite surface: POST /v1/commerce/invite/redeem (org
	// claims a code) + POST /v1/commerce/invite (platform-admin mint). NOT behind
	// requireAccess — redeeming a code is how an org acquires access.
	inviteApi.Route(api, tokenRequired)
	deployApi.Route(api, tokenRequired)
	formApi.Route(api, tokenRequired)
	inventoryApi.Route(api, tokenRequired, requireAccess)
	orderApi.Route(api, tokenRequired, requireAccess)
	referralApi.Route(api, tokenRequired)
	payablesApi.Route(api, adminRequired) // what we owe + manual payment records (RequirePlatformAdmin inside)
	affiliateApi.Route(api, tokenRequired)
	regionApi.Route(api, tokenRequired)
	reviewApi.Route(api, tokenRequired)
	storeApi.Route(api, tokenRequired)
	transactionApi.Route(api, tokenRequired)
	uploadApi.Route(api, adminRequired) // admin product-image uploads → S3 (tenant-prefixed, image-only)
	userApi.Route(api, tokenRequired)
	pricingApi.Route(api, tokenRequired, requireAccess)
	promotionApi.Route(api, tokenRequired, requireAccess)
	promoApi.Route(api) // SuperAdmin plan-promo config (/v1/platform/promo) — RequirePlatformAdmin inside

	// Medusa-parity admin domains. These sub-routers were fully implemented
	// (models orm.Register'd, tenant-scoped via middleware.Namespace) but never
	// wired into the /v1 bundle, so their routes 404'd in production. Wiring
	// them here completes the fulfillment/shipping, tax, customer-group,
	// publishable-api-key/RBAC, and notification admin domains.
	fulfillmentApi.Route(api, tokenRequired, requireAccess) // fulfillment sets/providers, shipping options/profiles, service+geo zones, ship/cancel
	taxApi.Route(api, tokenRequired, requireAccess)         // tax regions/rates/rules/providers + /tax/calculate
	customergroupApi.Route(api, tokenRequired)
	apikeyApi.Route(api, tokenRequired) // publishable API keys, roles, api permissions
	notificationApi.Route(api, tokenRequired)
	giftcardApi.Route(api, adminRequired, requireAccess)   // gift cards + idempotent redeem/void (money — admin only)
	b2bApi.Route(api, tokenRequired, requireAccess)        // B2B: companies, employees, quotes, approvals
	exchangeApi.Route(api, tokenRequired)                  // order exchanges (return + replacement)
	currencyApi.Route(api, adminRequired)                  // currency reference table CRUD (global default-ns; public list on commerce group)
	claimApi.Route(api, tokenRequired)                     // order claims (damaged/wrong/missing → refund or replacement)
	draftorderApi.Route(api, tokenRequired, requireAccess) // admin order builder: draft orders + line items → complete into a real order
	producttaxonomyApi.Route(api, tokenRequired)           // product options/values, categories, tags, types, return/refund reasons
	catalogApi.AdminRoute(api, adminRequired)              // platform product catalog CMS (global-admin gated inside)
	// Public catalog projection GET /v1/commerce/catalog is wired on the
	// commerce public group (commerce.go) so it serves that exact path.

	// Platform subscription/DNS plan authority CRUD (global-admin gated inside).
	// Public read stays GET /v1/billing/plans; the embed seed source is injected
	// here (the composition root) so api/plan stays free of an api/billing import.
	planApi.AdminRoute(api, billingApi.SeedRows, adminRequired)

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

	// Mercury bank webhook
	mercuryApi.Route(api)

	// Extra routes registered by optional sub-modules (e.g. luxfi/cevm)
	applyExtraRoutes(api)
}
