package namespace

import "github.com/gin-gonic/gin"

func Route(router *gin.RouterGroup, args ...gin.HandlerFunc) {
	router.GET("/c/namespace/by-id/:id", namespaceFromId)
	router.GET("/c/namespace/to-id/:namespace", idFromNamespace)
}
