package api

import (
	"crowdstart.io/api/cart"
	"crowdstart.io/api/user"
	"crowdstart.io/api/order"
	"crowdstart.io/api/product"
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	api := router.Group("/v1")

	// Cart API
	api.GET("/cart/:id", cart.Get)
	api.POST("/cart", cart.Add)
	api.PUT("/cart", cart.Update)
	api.DELETE("/cart", cart.Delete)

	api.GET("/user/:id", user.Get)
	api.POST("/user", user.Add)
	api.PUT("/user", user.Update)
	api.DELETE("/user", user.Delete)

	api.GET("/order/:id", order.Get)
	api.POST("/order", order.Add)
	api.PUT("/order", order.Update)
	api.DELETE("/order", order.Delete)

	api.GET("/product/:id", product.Get)
	api.POST("/product", product.Add)
	api.PUT("/product", product.Update)
	api.DELETE("/product", product.Delete)

	// Redirect root
	api.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://crowdstart.io")
	})

	http.Handle("/v1/", router)
}
