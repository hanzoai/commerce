package middleware

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

// Serve custom 404 page.
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Request.Method == "GET" && !c.Writer.Written() && c.Writer.Status() == 404 {
			http.ServeFile(c.Writer, c.Request, "../static/404.html")
		}
	}
}

