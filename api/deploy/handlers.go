package site

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("site")

	// Deploys
	api.Get("/:siteid/deploy", adminRequired, listDeploys)
	api.Get("/:siteid/deploy/:deployid", adminRequired, getDeploy)
	api.Post("/:siteid/deploy", adminRequired, createDeploy)
	api.Get("/:siteid/deploy/:deployid/restore", adminRequired, restoreDeploy)
	api.Put("/:siteid/deploy/:deployid/files/*filepath", adminRequired, putFile)

	// Files
	api.Get("/:siteid/file", adminRequired, listFiles)
	api.Get("/:siteid/file/*filepath", adminRequired, getFile)
}
