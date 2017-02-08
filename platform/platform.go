package platform

import (
	"hanzo.io/middleware"
	"hanzo.io/platform/admin"
	"hanzo.io/platform/frontend"
	"hanzo.io/platform/login"
	"hanzo.io/platform/user"
	"hanzo.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	loginRequired := middleware.LoginRequired("platform")
	logoutRequired := middleware.LogoutRequired("platform")
	acquireUser := middleware.AcquireUser("platform")
	acquireOrganization := middleware.AcquireOrganization("platform")

	// Frontend
	// router.GET("/", frontend.Index)
	router.GET("/", loginRequired, acquireUser, acquireOrganization, admin.Dashboard)
	// router.GET("/about", frontend.About)
	// router.GET("/contact", frontend.Contact)
	// router.GET("/faq", frontend.Faq)
	// router.GET("/features", frontend.Features)
	// router.GET("/how-it-works", frontend.HowItWorks)
	// router.GET("/pricing", frontend.Pricing)
	// router.GET("/privacy", frontend.Privacy)
	// router.GET("/team", frontend.Team)
	// router.GET("/terms", frontend.Terms)

	// Docs
	// router.GET("/docs", docs.GettingStarted)
	// router.GET("/docs/api", docs.API)
	// router.GET("/docs/checkout", docs.Checkout)
	// router.GET("/docs/crowdstart.js", docs.CrowdstartJS)
	// router.GET("/docs/salesforce", docs.Salesforce)

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

	dash.GET("/profile", user.Profile)
	dash.POST("/profile", user.ContactSubmit)
	dash.POST("/profile/password", user.PasswordSubmit)
	dash.GET("/keys", admin.Keys)
	dash.POST("/keys", admin.NewKeys)

	dash.GET("/sendorderconfirmation/:id", admin.SendOrderConfirmation)
	dash.GET("/sendrefundconfirmation/:id", admin.SendRefundConfirmation)
	dash.GET("/sendfulfillmentconfirmation/:id", admin.SendFulfillmentConfirmation)

	dash.GET("/organization", admin.Organization)
	dash.POST("/organization", admin.UpdateOrganization)

	dash.GET("/organization/:organizationid/set-active", admin.SetActiveOrganization)

	dash.GET("/settings", user.Profile)

	dash.GET("/search", admin.Search)

	// Stripe connect
	dash.GET("/stripe", admin.Stripe)
	dash.GET("/stripe/callback", admin.StripeCallback)
	dash.GET("/stripe/sync", admin.StripeSync)

	// Salesfoce connect
	dash.GET("/salesforce/callback", admin.SalesforceCallback)
	dash.GET("/salesforce/test", admin.TestSalesforceConnection)
	router.GET("/salesforce/sync", admin.SalesforcePullLatest)
}
