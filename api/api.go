package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/cart"
	"crowdstart.io/api/order"
	"crowdstart.io/api/product"
	"crowdstart.io/api/token"
	"crowdstart.io/api/user"
	"crowdstart.io/api/variant"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("api")

	// Redirect root
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/cart/:id", cart.Get)
	router.POST("/cart", cart.Add)
	router.PUT("/cart/:id", cart.Update)
	router.DELETE("/cart/:id", cart.Delete)

	router.GET("/user/:id", user.Get)
	router.POST("/user", user.Add)
	router.PUT("/user/:id", user.Update)
	router.DELETE("/user/:id", user.Delete)

	router.GET("/order/:id", order.Get)
	router.POST("/order", order.Add)
	router.PUT("/order/:id", order.Update)
	router.DELETE("/order/:id", order.Delete)

	router.GET("/product/:id", product.Get)
	router.POST("/product", product.Add)
	router.PUT("/product/:id", product.Update)
	router.DELETE("/product/:id", product.Delete)

	router.GET("/variant/:id", variant.Get)
	router.POST("/variant", variant.Add)
	router.PUT("/variant/:id", variant.Update)
	router.DELETE("/variant/:id", variant.Delete)

	router.GET("/token", token.List)
	router.GET("/token/", token.List)
	router.GET("/token/:id", token.Get)
	// router.POST("/variant", variant.Add)
	// router.PUT("/variant/:id", variant.Update)
	// router.DELETE("/variant/:id", variant.Delete)
}
