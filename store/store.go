package store

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"crowdstart.io/middleware"
)

func init() {
	router := gin.Default()

    router.Use(gin.Logger())
    router.Use(gin.Recovery())
    router.Use(middleware.AppEngine())

	router.GET("/foo/", func(ctx *gin.Context) {
		ctx.String(200, "foo")
	})

	router.GET("/bar/", func(ctx *gin.Context) {
		ctx.String(200, "bar")
	})

	http.Handle("/", router)
}
