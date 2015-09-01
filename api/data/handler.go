package data

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := router.Group("/c/data")
	api.Use(middleware.AccessControl("*"))

	api.GET("/dashboard/:period/:year/:month/:day/:tzOffset", adminRequired, namespaced, dashboard)
}
