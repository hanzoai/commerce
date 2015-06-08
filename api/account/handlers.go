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

	group := router.Group("account")
	group.Use(publishedRequired)

	group.GET("", publishedRequired, accountRequired, namespaced, get)
	group.PUT("", publishedRequired, accountRequired, namespaced, update)
	group.PATCH("", publishedRequired, accountRequired, namespaced, patch)

	group.POST("login", publishedRequired, namespaced, login)
	group.POST("new", publishedRequired, namespaced, create)
}
