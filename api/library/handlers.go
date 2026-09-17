package library

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := router.Group("library")

	api.Raw(http.MethodPost, "/shopjs", publishedRequired, namespaced, LoadShopJS)
	api.Raw(http.MethodPost, "/coinjs", publishedRequired, namespaced, LoadShopJS)
	api.Raw(http.MethodPost, "/daisho", LoadDaisho)
}
