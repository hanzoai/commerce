package middleware

import (
	"appengine"
	"github.com/gin-gonic/gin"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Automatically get the Host header so we can decide what to do with a given
// request.
func AddHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("host", c.Request.Header.Get("Host"))
	}
}

func NewRouter() *gin.Engine {
	router := gin.New()

	router.Use(ErrorHandler())
	router.Use(NotFoundHandler())
	router.Use(AddHost())
	router.Use(AppEngine())
	return router
}

func GetAppEngine(c *gin.Context) appengine.Context {
	return c.MustGet("appengine").(appengine.Context)
}
