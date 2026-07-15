package middleware

import "github.com/zap-proto/zip"

func AccessControl(allowOrigin string) zip.Handler {
	return func(c *zip.Ctx) error {
		// Set CORS headers for all requests.
		c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
		c.SetHeader("Access-Control-Allow-Credentials", "true")

		if c.Method() != "OPTIONS" {
			return c.Next()
		}

		// Handle preflight OPTIONS request
		reqHeaders := c.Header("Access-Control-Request-Headers")

		c.SetHeader("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.SetHeader("Access-Control-Allow-Headers", reqHeaders)
		c.SetHeader("Access-Control-Max-Age", "86400")

		return c.NoContent(204)
	}
}
