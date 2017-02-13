package dash

import (
	"hanzo.io/dash/api"
	"hanzo.io/dash/login"
	"hanzo.io/dash/user"
	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("dash")

	loginRequired := middleware.LoginRequired("dash")
	logoutRequired := middleware.LogoutRequired("dash")
	acquireUser := middleware.AcquireUser("dash")
	acquireOrganization := middleware.AcquireOrganization("dash")

	// Frontend
	// router.GET("/", frontend.Index)
	router.GET("/", loginRequired, acquireUser, acquireOrganization, api.Dashboard)
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
	// router.GET("/docs/hanzo.js", docs.HanzoJS)
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

	// api dashboard
	dash := router.Group("")
	dash.Use(loginRequired, acquireUser, acquireOrganization)

	dash.GET("/profile", user.Profile)
	dash.POST("/profile", user.ContactSubmit)
	dash.POST("/profile/password", user.PasswordSubmit)
	dash.GET("/keys", api.Keys)
	dash.POST("/keys", api.NewKeys)

	dash.GET("/sendorderconfirmation/:id", api.SendOrderConfirmation)
	dash.GET("/sendrefundconfirmation/:id", api.SendRefundConfirmation)
	dash.GET("/sendfulfillmentconfirmation/:id", api.SendFulfillmentConfirmation)
	dash.POST("/shipwire/ship/:id", api.ShipOrderUsingShipwire)
	dash.POST("/shipwire/return/:id", api.ReturnOrderUsingShipwire)

	dash.GET("/organization", api.Organization)
	dash.POST("/organization", api.UpdateOrganization)

	dash.GET("/organization/:organizationid/set-active", api.SetActiveOrganization)

	dash.GET("/settings", user.Profile)

	dash.GET("/search", api.Search)

	// Stripe connect
	dash.GET("/stripe", api.Stripe)
	dash.GET("/stripe/callback", api.StripeCallback)
	dash.GET("/stripe/sync", api.StripeSync)

	// Salesfoce connect
	dash.GET("/salesforce/callback", api.SalesforceCallback)
	dash.GET("/salesforce/test", api.TestSalesforceConnection)
	router.GET("/salesforce/sync", api.SalesforcePullLatest)
}
