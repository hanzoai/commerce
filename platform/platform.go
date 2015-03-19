package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/platform/frontend"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	loginRequired := middleware.LoginRequired("platform")

	router.GET("/", frontend.Index)
	router.GET("/about", frontend.About)
	router.GET("/contact", frontend.Contact)
	router.GET("/docs", frontend.Docs)
	router.GET("/faq", frontend.Faq)
	router.GET("/features", frontend.Features)
	router.GET("/pricing", frontend.Pricing)
	router.GET("/privacy", frontend.Privacy)
	router.GET("/team", frontend.Team)
	router.GET("/terms", frontend.Terms)

	router.GET("/theme/", admin.ThemeSample)

	router.GET("/dashboard", loginRequired, admin.Dashboard)

	router.GET("/login", admin.Login)
	router.POST("/login", admin.SubmitLogin)
	router.GET("/logout", admin.Logout)

	// router.GET("/register", admin.Register)
	// router.POST("/register", admin.SubmitRegister)

	router.GET("/profile", loginRequired, admin.Profile)
	router.POST("/profile", admin.SubmitProfile)

	router.GET("/organization", loginRequired, admin.Organization)
	router.GET("/keys", loginRequired, admin.Keys)

	router.GET("/settings", loginRequired, admin.Profile)

	// Stripe connect
	router.GET("/stripe/connect", loginRequired, admin.StripeConnect)
	router.GET("/stripe/callback", loginRequired, admin.StripeCallback)
	router.POST("/stripe/hook", stripe.StripeWebhook)
	router.GET("/stripe/sync", admin.StripeSync)

	// Salesfoce connect
	router.GET("/salesforce/callback", loginRequired, admin.SalesforceCallback)
	router.GET("/salesforce/test", loginRequired, admin.TestSalesforceConnection)
	router.GET("/salesforce/sync", admin.SalesforcePullLatest)
}
