package dashv2

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	// "github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	// publishedRequired := middleware.TokenRequired(permission.Admin)
	origin := middleware.AccessControl("*")

	api := router.Group("dashv2")
	api.Use(origin)
	api.Raw(http.MethodPost, "/login", login)
}
