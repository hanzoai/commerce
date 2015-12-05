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

	api.GET("/", adminRequired, list)
	api.GET("/:siteid", adminRequired, get)
	api.POST("/", adminRequired, create)
	api.PATCH("/:siteid", adminRequired, update)
	api.PUT("/:siteid", adminRequired, update)
	api.DELETE("/:siteid", adminRequired, delete)
}
