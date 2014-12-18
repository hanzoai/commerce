package store

import (
	"crowdstart.io/middleware"
	"crowdstart.io/store/card"
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

	// Password Reset
	router.GET("/create-password", user.CreatePassword)
	router.GET("/password-reset", user.PasswordReset)
	router.POST("/password-reset", user.PasswordResetSubmit)
	router.GET("/password-reset/:token", user.PasswordResetConfirm)
	router.POST("/password-reset/:token", user.PasswordResetConfirmSubmit)

	// Register
	router.GET("/register", logoutRequired, user.Register)
	router.POST("/register", logoutRequired, user.SubmitRegister)

	// Profile
	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile/:form", loginRequired, user.SaveProfile)

	router.GET("/orders", loginRequired, user.ListOrders)

	// Card
	router.GET("/card", loginRequired, card.GetCard)
}
