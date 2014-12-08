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

	loginRequired := middleware.LoginRequired("store")
	logoutRequired := middleware.LogoutRequired("store")
	checkLogin := middleware.CheckLogin()

	// Products
	router.GET("/", checkLogin, products.List)
	router.GET("/products", checkLogin, products.List)
	router.GET("/products/:slug", checkLogin, products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	// Login
	router.GET("/login", logoutRequired, user.Login)
	router.POST("/login", logoutRequired, user.SubmitLogin)
	router.GET("/logout", user.Logout)

	// Register
	router.GET("/register", logoutRequired, user.Register)
	router.POST("/register", logoutRequired, user.SubmitRegister)

	// Profile
	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile", loginRequired, user.SaveProfile)
}
