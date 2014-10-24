package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func init() {
	router := gin.Default()

    router.Use(gin.Logger())
    router.Use(gin.Recovery())

	router.GET("/v1/", func(ctx *gin.Context) {
		ctx.String(200, "api")
	})

	http.Handle("/v1/", router)
}
