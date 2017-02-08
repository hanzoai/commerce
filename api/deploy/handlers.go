package site

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("site")

	// Deploys
	api.GET("/:siteid/deploy", adminRequired, listDeploys)
	api.GET("/:siteid/deploy/:deployid", adminRequired, getDeploy)
	api.POST("/:siteid/deploy", adminRequired, createDeploy)
	api.GET("/:siteid/deploy/:deployid/restore", adminRequired, restoreDeploy)
	api.PUT("/:siteid/deploy/:deployid/files/*filepath", adminRequired, putFile)

	// Files
	api.GET("/:siteid/file", adminRequired, listFiles)
	api.GET("/:siteid/file/*filepath", adminRequired, getFile)
}
