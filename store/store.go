package store

import (
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "product listing")
	})

	http.Handle("/", router)
}
