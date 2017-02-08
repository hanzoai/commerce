package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
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

	api.GET("/order/:orderid", publishedRequired, accountRequired, namespaced, getOrder)
	api.PATCH("/order/:orderid", publishedRequired, accountRequired, namespaced, patchOrder)

	api.GET("/exists/:email", publishedRequired, namespaced, exists)

	api.POST("/login", publishedRequired, namespaced, login)

	api.POST("/create", publishedRequired, namespaced, create)
	api.POST("/enable/:tokenid", publishedRequired, namespaced, enable)

	api.POST("/reset", publishedRequired, namespaced, reset)
	api.POST("/confirm/:tokenid", publishedRequired, namespaced, confirm)
}
