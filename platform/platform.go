package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/platform/frontend"
	"crowdstart.io/platform/user"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	loginRequired := middleware.LoginRequired("platform")

	router.GET("/", frontend.Index)
	router.GET("/theme/", admin.ThemeSample)

	router.GET("/dashboard", loginRequired, admin.Dashboard)

	router.GET("/login", user.Login)
	router.POST("/login", user.SubmitLogin)
	router.GET("/logout", user.Logout)

	// router.GET("/register", admin.Register)
	// router.POST("/register", admin.SubmitRegister)

	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile", user.SubmitProfile)
	router.POST("/resetpassword", user.ResetPassword)

	router.GET("/organization", loginRequired, admin.Organization)
	router.GET("/keys", loginRequired, admin.Keys)

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
