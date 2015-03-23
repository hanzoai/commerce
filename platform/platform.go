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
	router.GET("/login", logoutRequired, user.Login)
	router.POST("/login", logoutRequired, user.SubmitLogin)
	router.GET("/logout", user.Logout)

	// Signup
	// router.GET("/signup", login.Signup)
	// router.POST("/signup", login.SignupSubmit)

	// Password Reset
	// router.GET("/create-password", user.CreatePassword)
	router.GET("/password-reset", login.PasswordReset)
	router.POST("/password-reset", login.PasswordResetSubmit)
	router.GET("/password-reset/:token", login.PasswordResetConfirm)
	router.POST("/password-reset/:token", login.PasswordResetConfirmSubmit)

	// Admin
	router.GET("/dashboard", loginRequired, admin.Dashboard)

	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile", user.SubmitProfile)
	router.POST("/changepassword", user.SubmitProfile)
	router.GET("/keys", loginRequired, admin.Keys)
	router.POST("/keys", loginRequired, admin.NewKeys)

	router.GET("/orders", loginRequired, admin.Orders)
	router.GET("/products", loginRequired, admin.Products)
	router.GET("/product/:id", loginRequired, admin.Product)
	router.GET("/organization", loginRequired, admin.Organization)

	router.GET("/settings", loginRequired, user.Profile)

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
