package store

import (
	"crowdstart.io/middleware"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/store/user"
	"crowdstart.io/util/router"
)

var loginRequired = middleware.LoginRequired("store")
var logoutRequired = middleware.LogoutRequired("store")

func init() {
	router := router.New("store")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	router.GET("/login", logoutRequired, user.Login)
	router.POST("/login", logoutRequired, user.SubmitLogin)
	router.GET("/logout", user.Logout)

	router.GET("/register", logoutRequired, user.Register)
	router.POST("/register", logoutRequired, user.SubmitRegister)

	router.GET("/profile", loginRequired, user.Profile)
	router.POST("/profile", loginRequired, user.SaveProfile)
}
