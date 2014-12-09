package store

import (
	"crowdstart.io/middleware"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/store/user"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("store")

	// Middleware
	router.Use(middleware.CheckLogin())
	loginRequired := middleware.LoginRequired("store")
	logoutRequired := middleware.LogoutRequired("store")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	// Login
	router.GET("/login", logoutRequired, user.Login)
	router.POST("/login", logoutRequired, user.SubmitLogin)
	router.GET("/logout", user.Logout)
	router.GET("/forgotpassword", user.ForgotPassword)
	router.POST("/forgotpassword", user.SubmitForgotPassword)

	// Register
	router.GET("/register", logoutRequired, user.Register)
	router.POST("/register", logoutRequired, user.SubmitRegister)

	// Profile
	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile", loginRequired, user.SaveProfile)
}
