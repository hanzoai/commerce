package auth

import (
	"github.com/zap-proto/zip"
)

func Route(router zip.Router, args ...zip.Handler) {
	router.Post("/auth", credentials)
}
