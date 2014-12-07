package store

import (
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/store/user"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("store")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	router.GET("/login", user.Login)
	router.GET("/logout", user.Logout)
	router.POST("/login", user.SubmitLogin)
}
