package analytics

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

func init() {
	analytics := router.New("analytics")
	tokenRequired := middleware.TokenRequired()
	namespaced := middleware.Namespace()

	// Use permissive CORS policy for all API routes.
	analytics.Use(middleware.AccessControl("*"))
	analytics.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	analytics.POST("/:organizationid", create)
	analytics.POST("/", tokenRequired, namespaced, create)
	analytics.HEAD("/", router.Empty)
}
