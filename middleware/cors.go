package middleware

import "github.com/gin-gonic/gin"

func AccessControl(allowOrigin string) gin.HandlerFunc {
	return func(c *context.Context) {
		// Allow all CORS requests.
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)

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
		header.Set("Access-Control-Max-Age", "86400")

		c.Data(200, "text/plain", make([]byte, 0))
		c.AbortWithStatus(200)
	}
}
