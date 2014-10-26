package api

import (
	"crowdstart.io/api/cart"
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

	// Redirect root
	api.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://crowdstart.io")
	})

	http.Handle("/v1/", router)
}
