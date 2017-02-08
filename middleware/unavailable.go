package middleware

import (
	"errors"
	"net/http"

	"hanzo.io/util/template"
	"github.com/gin-gonic/gin"

	"appengine"
)

var template503 = `
<html>
	<head>
		<style>
			body {
				font-family:monospace;
				margin:20px;
			}
		</style>
	</head>
	<body>
		<h4>503 Service Unavailable (crowdstart/1.0.0)</h4>
		<p>Service termporarily unvailable.</p>
	</body>
</html>
`

// Serve custom 404 page.
func UnavailableHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithError(http.StatusServiceUnavailable, errors.New("Service temporarily unavailable."))

		c.Next()

		if appengine.IsDevAppServer() {
			c.Writer.Write([]byte(template503))
		} else {
			template.Render(c, "error/503.html")
		}
	}
}
