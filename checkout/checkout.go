package checkout

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func init() {
	router := gin.Default()

    router.Use(gin.Logger())
    router.Use(gin.Recovery())

	router.GET("/checkout/", func(ctx *gin.Context) {
		ctx.String(200, "checkout")
	})

	http.Handle("/checkout/", router)
}
