package cdn

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/router"

	"hanzo.io/cdn/analytics"
	"hanzo.io/cdn/mailinglist"
	"hanzo.io/cdn/native"
)

func init() {
	cdn := router.New("cdn")

	// Use permissive CORS policy for all API routes.
	cdn.Use(middleware.AccessControl("*"))
	cdn.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	cdn.GET("/a/:organizationid", analytics.Js)
	cdn.GET("/a/:organizationid/analytics.js", analytics.Js)
	cdn.GET("/a/:organizationid/js", analytics.Js)

	cdn.GET("/m/:mailinglistid/mailinglist.js", mailinglist.Js)
	cdn.GET("/m/:mailinglistid/js", mailinglist.Js)

	cdn.GET("/n/:organizationid/native.js", native.Js)

	cdn.HEAD("/", router.Empty)
	cdn.GET("/robots.txt", router.Robots)
}
