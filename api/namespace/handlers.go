package namespace

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	// Namespace lookup exposes org names/IDs -- require admin token.
	adminRequired := middleware.TokenRequired(permission.Admin)

	router.Get("/c/namespace/by-id/:id", adminRequired, namespaceFromId)
	router.Get("/c/namespace/to-id/:namespace", adminRequired, idFromNamespace)
}
