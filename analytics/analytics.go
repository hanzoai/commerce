package analytics

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func init() {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	analytics := router.New("analytics")

	// Use permissive CORS policy for all API routes.
	analytics.Use(middleware.AccessControl("*"))
	analytics.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	analytics.GET("/native/js", publishedRequired, namespaced, js)
	analytics.POST("/", publishedRequired, namespaced, create)
	analytics.HEAD("/", router.Empty)
}
