package analytics

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
)

func init() {
	analytics := router.New("analytics")

	// Use permissive CORS policy for all API routes.
	analytics.Use(middleware.AccessControl("*"))
	analytics.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	analytics.POST("/:organizationid", create)
	analytics.HEAD("/", router.Empty)
}
