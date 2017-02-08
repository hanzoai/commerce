package dashboard

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	origin := middleware.AccessControl("*")

	api := router.Group("c")
	api.Use(origin)

	// api.GET("/dashboard", middleware.AccessTokenOnly, dashboard)

	// api.POST("/account/login", login)

	// api.POST("/account/create", create)
	// api.POST("/account/enable/:tokenid", enable)

	// api.POST("/account/reset", reset)
	// api.POST("/account/confirm/:tokenid", confirm)
}
