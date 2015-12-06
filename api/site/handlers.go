package site

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("site")

	// Sites
	api.GET("/", adminRequired, listSites)
	api.GET("/:siteid", adminRequired, getSite)
	api.POST("/", adminRequired, createSite)
	api.PATCH("/:siteid", adminRequired, updateSite)
	api.PUT("/:siteid", adminRequired, updateSite)
	api.DELETE("/:siteid", adminRequired, deleteSite)

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
