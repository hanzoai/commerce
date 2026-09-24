package api

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// Wire up Bitcoin endpoint
func Route(router *zip.Group, args ...zip.Handler) {
	api := router.Group("bitcoin")
	api.Raw(http.MethodPost, "/webhook", Webhook)
}
