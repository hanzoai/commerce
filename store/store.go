package store

import (
	"appengine"
	"encoding/gob"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

func init() {
	staticRoot, _ := os.Getwd()

	gob.Register(&LineItem{})
	gob.Register(&Cart{})

	router := gin.Default()

    router.Use(gin.Logger())
    router.Use(gin.Recovery())

    // Static files
	router.Static("/static", staticRoot)
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
