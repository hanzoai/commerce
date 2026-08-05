package api

import (
	"github.com/zap-proto/zip"
)

// Route registers Mercury webhook endpoint.
func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("mercury")
	api.Post("/webhook", Webhook)
}
