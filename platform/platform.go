package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	router := router.New("platform")

	router.GET("/", admin.Index)
	router.GET("/dashboard", middleware.LoginRequired(), admin.Dashboard)

	router.GET("/login", admin.Login)
	router.POST("/login", admin.SubmitLogin)
	router.GET("/logout", admin.Logout)

	router.GET("/register", admin.Register)
	router.POST("/register", admin.SubmitRegister)

	router.GET("/profile", middleware.LoginRequired(), admin.Profile)
	router.POST("/profile", admin.SubmitProfile)

	router.GET("/dashboard", middleware.LoginRequired(), admin.Dashboard)
	router.GET("/connect", middleware.LoginRequired(), admin.Connect)

	router.GET("/stripe/callback", middleware.LoginRequired(), admin.StripeCallback)
}
