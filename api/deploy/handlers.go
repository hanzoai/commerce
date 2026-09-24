package site

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("site")

	// Deploys
	api.Raw(http.MethodGet, "/:siteid/deploy", adminRequired, listDeploys)
	api.Raw(http.MethodGet, "/:siteid/deploy/:deployid", adminRequired, getDeploy)
	api.Raw(http.MethodPost, "/:siteid/deploy", adminRequired, createDeploy)
	api.Raw(http.MethodGet, "/:siteid/deploy/:deployid/restore", adminRequired, restoreDeploy)
	api.Raw(http.MethodPut, "/:siteid/deploy/:deployid/files/*filepath", adminRequired, putFile)

	// Files
	api.Raw(http.MethodGet, "/:siteid/file", adminRequired, listFiles)
	api.Raw(http.MethodGet, "/:siteid/file/*filepath", adminRequired, getFile)
}
