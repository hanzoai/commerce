package data

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := router.Group("/c/data")
	api.Use(middleware.AccessControl("*"))

	api.Raw(http.MethodGet, "/dashboard/:period/:year/:month/:day/:tzOffset", adminRequired, namespaced, dashboard)
}
