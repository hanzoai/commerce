package dashv2

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	// "github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	// publishedRequired := middleware.TokenRequired(permission.Admin)
	origin := middleware.AccessControl("*")

	api := router.Group("dashv2")
	api.Use(origin)
	api.POST("/login", login)
}
