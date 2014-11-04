package api

import (
	"crowdstart.io/api/cart"
	"crowdstart.io/api/order"
	"crowdstart.io/api/product"
	"crowdstart.io/api/user"
	"crowdstart.io/api/variant"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func init() {
	api := router.New("/v1/")

	// Redirect root
	api.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://crowdstart.io")
	})

	api.GET("/cart/:id", cart.Get)
	api.POST("/cart", cart.Add)
	api.PUT("/cart/:id", cart.Update)
	api.DELETE("/cart/:id", cart.Delete)

	api.GET("/user/:id", user.Get)
	api.POST("/user", user.Add)
	api.PUT("/user/:id", user.Update)
	api.DELETE("/user/:id", user.Delete)

	api.GET("/order/:id", order.Get)
	api.POST("/order", order.Add)
	api.PUT("/order/:id", order.Update)
	api.DELETE("/order/:id", order.Delete)

	api.GET("/product/:id", product.Get)
	api.POST("/product", product.Add)
	api.PUT("/product/:id", product.Update)
	api.DELETE("/product/:id", product.Delete)

	api.GET("/variant/:id", variant.Get)
	api.POST("/variant", variant.Add)
	api.PUT("/variant/:id", variant.Update)
	api.DELETE("/variant/:id", variant.Delete)
}
