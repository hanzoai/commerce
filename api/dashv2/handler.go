package dashv2

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	// "hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	// publishedRequired := middleware.TokenRequired(permission.Admin)
	origin := middleware.AccessControl("*")

	api := router.Group("dashv2")
	api.Use(origin)
	api.PUT("/login", login)
}
