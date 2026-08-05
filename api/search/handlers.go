package search

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	group := router.Group("search")
	group.Use(middleware.AccessControl("*"))

	group.Get("/user", adminRequired, namespaced, searchUser)
	group.Get("/order", adminRequired, namespaced, searchOrder)
	group.Post("/note", adminRequired, namespaced, searchNote)
}
