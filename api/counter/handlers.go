package counter

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	namespaced := middleware.Namespace()
	origin := middleware.AccessControl("*")

	api := router.Group("counter")
	api.Use(origin)

	api.Post("", adminRequired, namespaced, search)
	api.Post("/dashboard/daily", adminRequired, namespaced, daily)
	api.Get("/product/:productid", publishedRequired, namespaced, searchProduct)
	api.Get("/topline", publishedRequired, namespaced, topLine)
}
