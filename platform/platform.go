package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/platform/admin"
	"crowdstart.io/platform/user"
	"crowdstart.io/util/router"
)

// Defines the routes for the platform
func init() {
	adminR := router.New("/admin/")
	userR := router.New("/user/")

	userR.GET("/login", user.Login)

	adminR.GET("/", admin.Index)

	adminR.GET("/register", admin.Register)
	adminR.POST("/register", admin.SubmitRegister)

	adminR.GET("/login", admin.Login)
	adminR.POST("/login", admin.SubmitLogin)

	adminR.GET("logout", admin.Logout)

	adminR.GET("/profile", middleware.LoginRequired(), admin.Profile)
	adminR.POST("/profile", admin.SubmitProfile)

	adminR.GET("/dashboard", middleware.LoginRequired(), admin.Dashboard)
	adminR.GET("/connect", middleware.LoginRequired(), admin.Connect)

	adminR.GET("/stripe/callback", middleware.LoginRequired(), admin.StripeCallback)
}
