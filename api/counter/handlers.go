package counter

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	namespaced := middleware.Namespace()
	origin := middleware.AccessControl("*")

	api := router.Group("counter")
	api.Use(origin)

	api.Raw(http.MethodPost, "", adminRequired, namespaced, search)
	api.Raw(http.MethodPost, "/dashboard/daily", adminRequired, namespaced, daily)
	api.Raw(http.MethodGet, "/product/:productid", publishedRequired, namespaced, searchProduct)
	api.Raw(http.MethodGet, "/topline", publishedRequired, namespaced, topLine)
}
