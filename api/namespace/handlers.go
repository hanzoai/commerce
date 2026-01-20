package namespace

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	router.GET("/c/namespace/by-id/:id", namespaceFromId)
	router.GET("/c/namespace/to-id/:namespace", idFromNamespace)
}
