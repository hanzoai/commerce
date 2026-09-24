package middleware

import (
	"errors"
	"os"

	"github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/zip"
)

// Serve custom 404 page.
//
// fiber returns fiber.ErrNotFound from c.Next() when no route matched; that is
// the ONE signal for a framework 404. A handler that renders its own 404 (via
// http.Fail) returns nil, so it is left untouched — only the unmatched-route
// case is replaced with the custom page.
func NotFoundHandler() zip.Handler {
	return func(c *zip.Ctx) error {
		err := c.Next()
		if err == nil || !errors.Is(err, fiber.ErrNotFound) {
			return err
		}

		c.SetHeader("Content-Type", "text/html")

		// Simple 404 response (can be enhanced with template support)
		if os.Getenv("ENV") == "development" {
			return c.Bytes(404, []byte("<head><style>body{font-family:monospace; margin:20px}</style><h4>404 Not Found </h1><p>No such file or directory.</p>"))
		}
		return c.Bytes(404, []byte("<head><style>body{font-family:monospace; margin:20px}</style><h4>404 Not Found</h1><p>No such file or directory.</p>"))
	}
}
