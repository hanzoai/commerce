package middleware

import (
	"net/url"

	"github.com/gin-gonic/gin"
)

func AccessControl(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow all CORS requests.
		domain, _ := url.Parse(c.Request.Referer())

		if allowOrigin == "*" {
			// We need to send back and ACTUAL origin instead of * for cookies to work
			origin := domain.Scheme + "://" + domain.Host
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method != "OPTIONS" {
			return
		}

		// Handle OPTIONS
		header := c.Request.Header

		// reqMethods := header.Get("Access-Control-Request-Method")
		// reqMethods := header.Get("Access-Control-Request-Methods")
		// reqHeaders := header.Get("Access-Control-Request-Header")
		reqHeaders := header.Get("Access-Control-Request-Headers")

		header = c.Writer.Header()
		header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		header.Set("Access-Control-Allow-Headers", reqHeaders)

		c.Data(200, "text/plain", make([]byte, 0))
		c.Abort(200)
	}
}
