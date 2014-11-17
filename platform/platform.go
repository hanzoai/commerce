package platform

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

type TokenData struct {
	Access_token           string
	Error                  string
	Error_description      string
	Livemode               bool
	Refresh_token          string
	Scope                  string
	Stripe_publishable_key string
	Stripe_user_id         string
	Token_type             string
}

// Defines the routes for the platform
func init() {
	admin := router.New("/admin/")
	router.New("/user/") // for future usage

	admin.GET("/", adminIndex)

	admin.GET("/register", adminRegister)
	admin.POST("/register", adminSubmitRegister)

	admin.GET("/login", adminLogin)
	admin.POST("/login", adminSubmitLogin)

	admin.GET("logout", adminLogout)

	admin.GET("/profile", middleware.LoginRequired(), adminProfile)
	admin.POST("/profile", adminSubmitProfile)

	admin.GET("/dashboard", middleware.LoginRequired(), adminDashboard)
	admin.GET("/connect", middleware.LoginRequired(), adminConnect)

	admin.GET("/stripe/callback", middleware.LoginRequired(), stripeCallback)
}
