package counter

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()
	origin := middleware.AccessControl("*")

	api := router.Group("counter")
	api.Use(adminRequired, origin)

	api.POST("", namespaced, search)
}
