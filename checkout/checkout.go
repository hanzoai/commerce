package checkout

import (
	"net/http"
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	checkout := router.Group("/checkout")

	checkout.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "checkout")
	})

	checkout.POST("/", func(ctx *gin.Context) {
		ctx.String(200, "checkout")
	})

	http.Handle("/checkout/", router)
}
