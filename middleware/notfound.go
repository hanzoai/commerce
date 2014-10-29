package middleware

import (
	"appengine"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Serve custom 404 page.
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.Writer.Written() && c.Writer.Status() == 404 {
			c.Writer.Header().Set("Content-Type", "text/html")
			c.Writer.WriteHeader(http.StatusNotFound)

			if appengine.IsDevAppServer() {
				c.Writer.Write([]byte("<head><style>body{font-family:monospace; margin:20px}</style><h4>404 Not Found (crowdstart/1.0.0)</h1><p>No matching handlers found.</p>"))
			} else {
				template.Render(c, "error/404.html")
			}
		}
	}
}
