package store

import (
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("/")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)
}
