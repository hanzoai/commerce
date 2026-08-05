package api

import (
	"github.com/zap-proto/zip"
)

// Wire up Bitcoin endpoint
func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("bitcoin")
	api.Post("/webhook", Webhook)
}
