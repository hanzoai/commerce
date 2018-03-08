package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"hanzo.io/util/template"

	"google.golang.org/appengine"
)

// Serve custom 404 page.
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.Writer.Written() && c.Writer.Status() == 404 {
			c.Writer.Header().Set("Content-Type", "text/html")
			c.Writer.WriteHeader(http.StatusNotFound)

			if appengine.IsDevAppServer() {
				c.Writer.Write([]byte("<head><style>body{font-family:monospace; margin:20px}</style><h4>404 Not Found (hanzo/1.0)</h1><p>No such file or directory.</p>"))
			} else {
				template.Render(c, "error/404.html")
			}
		}
	}
}
