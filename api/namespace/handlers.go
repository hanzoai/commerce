package namespace

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	// Namespace lookup exposes org names/IDs -- require admin token.
	adminRequired := middleware.TokenRequired(permission.Admin)

	router.Raw(http.MethodGet, "/c/namespace/by-id/:id", adminRequired, namespaceFromId)
	router.Raw(http.MethodGet, "/c/namespace/to-id/:namespace", adminRequired, idFromNamespace)
}
