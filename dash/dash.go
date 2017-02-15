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

	// Dashboard
	router.GET("/", loginRequired, acquireUser, acquireOrganization, api.Dashboard)

	// Login
	router.GET("/login", logoutRequired, login.Login)
	router.POST("/login", logoutRequired, login.LoginSubmit)
	router.GET("/logout", login.Logout)

	// Password Reset
	router.GET("/password-reset", login.PasswordReset)
	router.POST("/password-reset", login.PasswordResetSubmit)
	router.GET("/password-reset/:token", login.PasswordResetConfirm)
	router.POST("/password-reset/:token", login.PasswordResetConfirmSubmit)

	// Dashboard routes
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
