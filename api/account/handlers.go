package account

import (
	"net/url"

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
	api.Use(
		func(c *gin.Context) {
			domain, _ := url.Parse(c.Request.Referer())
			origin := domain.Scheme + "://" + domain.Host
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		},
		publishedRequired)

	api.GET("", publishedRequired, accountRequired, namespaced, get)
	api.PUT("", publishedRequired, accountRequired, namespaced, update)
	api.PATCH("", publishedRequired, accountRequired, namespaced, patch)

	api.POST("/login", publishedRequired, namespaced, login)
	api.POST("/create", publishedRequired, namespaced, create)
}
