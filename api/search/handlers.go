package search

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	group := router.Group("search")
	group.Use(middleware.AccessControl("*"))

	group.GET("/user", adminRequired, namespaced, searchUser)
	group.GET("/order", adminRequired, namespaced, searchOrder)
	group.POST("/note", adminRequired, namespaced, searchNote)
}
