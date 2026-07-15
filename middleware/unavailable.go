package middleware

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/util/template"
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
		<h4>503 Service Unavailable </h4>
		<p>Service termporarily unvailable.</p>
	</body>
</html>
`

// Serve custom 503 page.
func UnavailableHandler() zip.Handler {
	return func(c *zip.Ctx) error {
		if config.IsDevelopment {
			return c.Bytes(503, []byte(template503))
		}
		return template.Render(c, "error/503.html")
	}
}
