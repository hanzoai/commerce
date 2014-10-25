package api

import (
	"crowdstart.io/api/cart"
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.Host())

	api := router.Group("/v1")

	api.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://crowdstart.io")
	})

	api.GET("/cart/:id", cart.Get)
	api.POST("/cart",	 cart.Add)
	api.PUT("/cart",	 cart.Update)
	api.DELETE("/cart",  cart.Delete)

	http.Handle("/v1/", router)
}
