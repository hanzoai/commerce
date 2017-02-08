package search

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(r router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Published)
	namespaced := middleware.Namespace()

	api := r.Group("search")
	// api.Use(middleware.AccessControl("*"))

	api.GET("/", adminRequired, namespaced, searchAll)
	api.GET("/:kind", adminRequired, namespaced, searchKind)
}
