package api

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// Route registers Mercury webhook endpoint.
func Route(router *zip.Group, args ...zip.Handler) {
	api := router.Group("mercury")
	api.Raw(http.MethodPost, "/webhook", Webhook)
}
