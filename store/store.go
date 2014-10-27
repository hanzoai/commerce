package store

import (
	"crowdstart.io/middleware"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"net/http"
)

func init() {
	router := middleware.NewRouter()

	// Products
	router.GET("/",				  products.List)
	router.GET("/products",		  products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	http.Handle("/", router)
}
