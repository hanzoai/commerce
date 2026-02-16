package namespace

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	// Namespace lookup exposes org names/IDs -- require admin token.
	adminRequired := middleware.TokenRequired(permission.Admin)

	router.GET("/c/namespace/by-id/:id", adminRequired, namespaceFromId)
	router.GET("/c/namespace/to-id/:namespace", adminRequired, idFromNamespace)
}
