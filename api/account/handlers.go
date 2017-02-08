package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()
	origin := middleware.AccessControl("*")

	api := router.Group("account")
	api.Use(publishedRequired, origin)

	// Customer Token Endpoints
	api.GET("", publishedRequired, namespaced, middleware.CustomerTokenOnly, get)
	api.PUT("", publishedRequired, namespaced, middleware.CustomerTokenOnly, update)
	api.PATCH("", publishedRequired, namespaced, middleware.CustomerTokenOnly, patch)

	api.GET("/order/:orderid", publishedRequired, namespaced, middleware.CustomerTokenOnly, getOrder)
	api.PATCH("/order/:orderid", publishedRequired, namespaced, middleware.CustomerTokenOnly, patchOrder)

	// Api Key Endpoints
	api.GET("/exists/:email", publishedRequired, namespaced, middleware.ApiKeyOnly, exists)

	api.POST("/login", publishedRequired, namespaced, middleware.ApiKeyOnly, login)

	api.POST("/create", publishedRequired, namespaced, middleware.ApiKeyOnly, create)
	api.POST("/enable/:tokenid", publishedRequired, namespaced, middleware.ApiKeyOnly, enable)

	api.POST("/reset", publishedRequired, namespaced, middleware.ApiKeyOnly, reset)
	api.POST("/confirm/:tokenid", publishedRequired, namespaced, middleware.ApiKeyOnly, confirm)
}
