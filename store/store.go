package store

import (
	"appengine"
	"crowdstart.io/datastore"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/store/cart"
	"crowdstart.io/store/products"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func init() {
	router := router.New("/")

	// Products
	router.GET("/", products.List)
	router.GET("/products", products.List)
	router.GET("/products/:slug", products.Get)

	// Cart
	router.GET("/cart", cart.Get)

	// Warmup, install fixtures, etc.
	router.GET("_ah/warmup", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		db := datastore.New(ctx)
		fixtures.Install(db)
	})
}
