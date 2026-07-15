package xd

import (
	"github.com/zap-proto/zip"
)

func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("/xd")

	api.Get("/:domain/proxy.html", func(c *zip.Ctx) error {
		c.SetHeader("Access-Control-Allow-Origin", "*")
		c.SetHeader("Content-Type", "text/html; charset=utf-8")

		domain := c.Param("domain")

		// Render response
		return c.Bytes(200, []byte(`<!DOCTYPE HTML>
<script src="//cdn.rawgit.com/jpillora/xdomain/0.7.4/dist/xdomain.min.js" master="https://`+domain+`">
</script>`))
	})
}
