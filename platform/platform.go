package platform

import (
	"crowdstart.com/middleware"
	"crowdstart.com/platform/admin"
	"crowdstart.com/platform/docs"
	"crowdstart.com/platform/frontend"
	"crowdstart.com/platform/login"
	"crowdstart.com/platform/user"
	stripe "crowdstart.com/thirdparty/stripe/webhook"
	"crowdstart.com/util/router"
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
	router.GET("/docs/checkout", docs.Checkout)
	router.GET("/docs/crowdstart.js", docs.CrowdstartJS)
	router.GET("/docs/salesforce", docs.Salesforce)

	// Login
	router.GET("/login", logoutRequired, login.Login)
	router.POST("/login", logoutRequired, login.LoginSubmit)
	router.GET("/logout", login.Logout)

	// Signup
	router.GET("/signup", frontend.Signup)
	// router.GET("/signup", login.Signup)
	// router.POST("/signup", login.SignupSubmit)

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

	dash.GET("/users", admin.Users)
	dash.GET("/user/:id", admin.User)

	dash.GET("/orders", admin.Orders)
	dash.GET("/order/:id", admin.Order)
	dash.GET("/sendorderconfirmation/:id", admin.SendOrderConfirmation)

	dash.GET("/mailinglists", admin.MailingLists)
	dash.GET("/mailinglist/:id", admin.MailingList)

	dash.GET("/products", admin.Products)
	dash.GET("/product/:id", admin.Product)

	dash.GET("/coupons", admin.Coupons)
	dash.GET("/coupon/:id", admin.Coupon)

	dash.GET("/stores", admin.Stores)
	dash.GET("/store/:id", admin.Store)

	dash.GET("/organization", admin.Organization)

	dash.GET("/settings", user.Profile)

	dash.GET("/search", admin.Search)

	// Stripe connect
	dash.GET("/stripe/connect", admin.StripeConnect)
	dash.GET("/stripe/callback", admin.StripeCallback)
	dash.GET("/stripe/sync", admin.StripeSync)
	router.POST("/stripe/hook", stripe.Webhook)

	// Salesfoce connect
	dash.GET("/salesforce/callback", admin.SalesforceCallback)
	dash.GET("/salesforce/test", admin.TestSalesforceConnection)
	router.GET("/salesforce/sync", admin.SalesforcePullLatest)
}
