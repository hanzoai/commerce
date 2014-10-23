package crowdstart

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"appengine"
)

func init() {
	router := gin.Default()

	router.Use(
		setCtx,
		CheckSession,
	)

	router.GET("/", func(ctx *gin.Context) {
		log.Println("Request on index")
		ctx.String(200, "Index")
	})

	http.Handle("/", router)
}

func setCtx(ctx *gin.Context) {
	c := appengine.NewContext(ctx.Request)
	ctx.Set("appengine_ctx", c)
	ctx.Next()
}
