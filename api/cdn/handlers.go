package cdn

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/cdn/analytics"
	"hanzo.io/api/cdn/mailinglist"
	"hanzo.io/api/cdn/native"
	"hanzo.io/middleware"
	"hanzo.io/util/router"
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

func group(r router.Router, prefix string) *gin.RouterGroup {
	g := r.Group(prefix)

	// Use permissive CORS policy for all API routes.
	g.Use(middleware.AccessControl("*"))
	g.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	return g
}

func Route(r router.Router, args ...gin.HandlerFunc) {
	a := group(r, "/a/")
	a.GET(":organizationid", analytics.Js)
	a.GET(":organizationid/analytics.js", analytics.Js)
	a.GET(":organizationid/js", analytics.Js)

	m := group(r, "/m/")
	m.GET(":mailinglistid/mailinglist.js", mailinglist.Js)
	m.GET(":mailinglistid/js", mailinglist.Js)

	n := group(r, "/n/")
	n.GET(":organizationid/native.js", native.Js)
}
