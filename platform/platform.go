package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	loginRequired := middleware.LoginRequired("platform")

	router.GET("/", admin.Index)

	router.GET("/dashboard", loginRequired, admin.Dashboard)

	router.GET("/login", admin.Login)
	router.POST("/login", admin.SubmitLogin)
	router.GET("/logout", admin.Logout)

	router.GET("/register", admin.Register)
	router.POST("/register", admin.SubmitRegister)

	router.GET("/profile", loginRequired, admin.Profile)
	router.POST("/profile", admin.SubmitProfile)

	router.GET("/connect", loginRequired, admin.Connect)

	// Stripe connect
	router.GET("/stripe/callback", loginRequired, admin.StripeCallback)

	// Stripe webhook, we don't do anything with this atm.
	router.POST("/stripe/hook", admin.StripeWebhook)

	// Salesfoce connect
	router.GET("/salesforce/callback", loginRequired, admin.SalesforceCallback)

	router.GET("/salesforce/test", loginRequired, admin.TestSalesforceConnection)
}
