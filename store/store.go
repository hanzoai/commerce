package store

import (
	"crowdstart.io/middleware"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	// Products
	router.GET("/",				  products.List)
	router.GET("/products",		  products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	http.Handle("/", router)
}
