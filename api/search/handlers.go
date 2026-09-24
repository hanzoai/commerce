package search

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	group := router.Group("search")
	group.Use(middleware.AccessControl("*"))

	group.Raw(http.MethodGet, "/user", adminRequired, namespaced, searchUser)
	group.Raw(http.MethodGet, "/order", adminRequired, namespaced, searchOrder)
	group.Raw(http.MethodPost, "/note", adminRequired, namespaced, searchNote)
}
