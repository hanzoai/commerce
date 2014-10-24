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
