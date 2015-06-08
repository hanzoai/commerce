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

	api := router.Group("account")
	api.Use(middleware.AccessControl("*"), publishedRequired)

	api.GET("", publishedRequired, accountRequired, namespaced, get)
	api.PUT("", publishedRequired, accountRequired, namespaced, update)
	api.PATCH("", publishedRequired, accountRequired, namespaced, patch)

	api.POST("/login", publishedRequired, namespaced, login)
	api.POST("/create", publishedRequired, namespaced, create)
}
