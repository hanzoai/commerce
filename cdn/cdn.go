package cdn

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
)

func init() {
	cdn := router.New("cdn")

	// Use permissive CORS policy for all API routes.
	cdn.Use(middleware.AccessControl("*"))
	cdn.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	cdn.GET("/:orgid/native/js", js)
	cdn.POST("/:orgid/", create)
	cdn.HEAD("/", router.Empty)
}
