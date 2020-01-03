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

	api := router.Group("library")

	api.POST("/shopjs", publishedRequired, namespaced, LoadShopJS)
	api.POST("/coinjs", publishedRequired, namespaced, LoadShopJS)
	api.POST("/daisho", LoadDaisho)
}
