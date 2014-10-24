package store

import (
	"appengine"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := gin.Default()

    router.Use(gin.Logger())
    router.Use(gin.Recovery())

	router.Use(setCtx)
	router.Use(CheckSession)

	router.GET("/foo/", func(ctx *gin.Context) {
		ctx.String(200, "foo")
	})

	router.GET("/bar/", func(ctx *gin.Context) {
		ctx.String(200, "bar")
	})

	http.Handle("/", router)
}

func setCtx(ctx *gin.Context) {
	c := appengine.NewContext(ctx.Request)
	ctx.Set("appengine_ctx", c)
	ctx.Next()
}
