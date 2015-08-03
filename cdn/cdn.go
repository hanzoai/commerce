package cdn

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/router"

	"crowdstart.com/cdn/analytics"
	"crowdstart.com/cdn/form"
	"crowdstart.com/cdn/native"
)

func init() {
	cdn := router.New("cdn")

	// Use permissive CORS policy for all API routes.
	cdn.Use(middleware.AccessControl("*"))
	cdn.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	cdn.GET("/:organizationid/v1/analytics.js", analytics.Js)
	cdn.GET("/:organizationid/v1/form.js", form.Js)
	cdn.GET("/:organizationid/v1/native.js", native.Js)

	cdn.HEAD("/", router.Empty)
}
