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
	api.POST("/", adminRequired, createSite)
	api.PATCH("/:siteid", adminRequired, updateSite)
	api.PUT("/:siteid", adminRequired, updateSite)
	api.DELETE("/:siteid", adminRequired, destroySite)
	api.GET("/", adminRequired, getAllSites)
	api.GET("/:siteid", adminRequired, getSingleSite)
}
