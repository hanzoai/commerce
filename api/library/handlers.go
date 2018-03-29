package library

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := router.Group("/library/")
	api.Use(publishedRequired)

	api.POST("shopjs", namespaced, LoadShopJS)
	api.POST("coinjs", namespaced, LoadShopJS)
}
