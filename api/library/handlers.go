package library

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := router.Group("library")

	api.Post("/shopjs", publishedRequired, namespaced, LoadShopJS)
	api.Post("/coinjs", publishedRequired, namespaced, LoadShopJS)
	api.Post("/daisho", LoadDaisho)
}
