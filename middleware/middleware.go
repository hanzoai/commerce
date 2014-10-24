package middleware

import (
	"appengine"
	"github.com/gin-gonic/gin"
	"time"
	"crowdstart.io/middleware/cookies"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Middleware for working with cookies
func CookieParser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		cookies := cookies.Parse(parser)
		c.Set("cookies", cookies)
		c := appengine.NewContext(ctx.Request)
	}
}
