package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/platform/docs"
	"crowdstart.io/platform/frontend"
	"crowdstart.io/platform/login"
	"crowdstart.io/platform/user"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	loginRequired := middleware.LoginRequired("platform")
	logoutRequired := middleware.LogoutRequired("platform")
	acquireUser := middleware.AcquireUser("platform")
	acquireOrganization := middleware.AcquireOrganization("platform")

	// Frontend
	router.GET("/", frontend.Index)
	router.GET("/about", frontend.About)
	router.GET("/contact", frontend.Contact)
	router.GET("/faq", frontend.Faq)
	router.GET("/features", frontend.Features)
	router.GET("/how-it-works", frontend.HowItWorks)
	router.GET("/pricing", frontend.Pricing)
	router.GET("/privacy", frontend.Privacy)
	router.GET("/team", frontend.Team)
	router.GET("/terms", frontend.Terms)

	// Docs
	router.GET("/docs", docs.GettingStarted)
	router.GET("/docs/api", docs.API)
	router.GET("/docs/crowdstart.js", docs.CrowdstartJS)
	router.GET("/docs/salesforce", docs.Salesforce)

	// Login
	router.GET("/login", logoutRequired, login.Login)
	router.POST("/login", logoutRequired, login.LoginSubmit)
	router.GET("/logout", login.Logout)

	// Signup
	router.GET("/signup", login.Signup)
	router.POST("/signup", login.SignupSubmit)

	// Password Reset
	// router.GET("/create-password", user.CreatePassword)
	router.GET("/password-reset", login.PasswordReset)
	router.POST("/password-reset", login.PasswordResetSubmit)
	router.GET("/password-reset/:token", login.PasswordResetConfirm)
	router.POST("/password-reset/:token", login.PasswordResetConfirmSubmit)

	// Admin dashboard
	dash := router.Group("")
	dash.Use(loginRequired, acquireUser, acquireOrganization)
	dash.GET("/dashboard", admin.Dashboard)

	dash.GET("/profile", user.Profile)
	dash.POST("/profile/contact", user.ContactSubmit)
	dash.POST("/profile/password", user.PasswordSubmit)
	dash.GET("/keys", admin.Keys)
	dash.POST("/keys", admin.NewKeys)

	dash.GET("/orders", admin.Orders)
	dash.GET("/order/:id", admin.Order)
	dash.GET("/products", admin.Products)
	dash.GET("/product/:id", admin.Product)
	dash.GET("/stores", admin.Stores)
	dash.GET("/store/:id", admin.Store)
	dash.GET("/organization", admin.Organization)

	dash.GET("/settings", user.Profile)

	// Stripe connect
	dash.GET("/stripe/connect", admin.StripeConnect)
	dash.GET("/stripe/callback", admin.StripeCallback)
	dash.GET("/stripe/sync", admin.StripeSync)
	router.POST("/stripe/hook", stripe.StripeWebhook)

	// Salesfoce connect
	dash.GET("/salesforce/callback", admin.SalesforceCallback)
	dash.GET("/salesforce/test", admin.TestSalesforceConnection)
	router.GET("/salesforce/sync", admin.SalesforcePullLatest)
}
