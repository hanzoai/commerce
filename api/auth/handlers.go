package auth

import (
	"net/http"

	"github.com/zap-proto/zip"
)

func Route(router *zip.Group, args ...zip.Handler) {
	router.Raw(http.MethodPost, "/auth", credentials)
}
