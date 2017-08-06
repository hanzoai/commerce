package analytics

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

func init() {
	analytics := router.New("analytics")

	// Use permissive CORS policy for all API routes.
	analytics.Use(middleware.AccessControl("*"))
	analytics.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	analytics.POST("/:organizationid", create)
	analytics.HEAD("/", router.Ok)
}
