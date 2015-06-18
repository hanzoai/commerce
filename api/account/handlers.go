package account

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	accountRequired := middleware.AccountRequired()
	namespaced := middleware.Namespace()
	origin := middleware.AccessControl("*")

	api := router.Group("account")
	api.Use(publishedRequired, origin)

	api.GET("", publishedRequired, accountRequired, namespaced, get)
	api.PUT("", publishedRequired, accountRequired, namespaced, update)
	api.PATCH("", publishedRequired, accountRequired, namespaced, patch)

	api.GET("/exists/:email", publishedRequired, namespaced, exists)

	api.POST("/login", publishedRequired, namespaced, login)

	api.POST("/create", publishedRequired, namespaced, create)
	api.GET("/create/confirm/:tokenid", publishedRequired, namespaced, createConfirm)
	api.POST("/create/confirm/:tokenid", publishedRequired, namespaced, createConfirm)

	api.GET("/reset", publishedRequired, namespaced, reset)
	api.POST("/reset/confirm/:tokenid", publishedRequired, namespaced, resetConfirm)
}
