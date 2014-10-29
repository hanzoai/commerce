package middleware

import (
	"appengine"
	"net/http"
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

// Serve custom 404 page.
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.Writer.Written() && c.Writer.Status() == 404 {
			c.Writer.WriteHeader(http.StatusNotFound)
			if appengine.IsDevAppServer() {
				c.String(404, "error 404: No matching handlers found.")
			} else {
				template.Render(c, "error/404.html")
			}
		}
	}
}
