package cdn

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/cdn/analytics"
	"hanzo.io/api/cdn/mailinglist"
	"hanzo.io/api/cdn/native"
	"hanzo.io/util/router"
)

func Route(r router.Router, args ...gin.HandlerFunc) {
	a := r.Group("/a/")
	a.GET(":organizationid", analytics.Js)
	a.GET(":organizationid/analytics.js", analytics.Js)
	a.GET(":organizationid/js", analytics.Js)

	m := r.Group("/m/")
	m.GET(":mailinglistid/mailinglist.js", mailinglist.Js)
	m.GET(":mailinglistid/js", mailinglist.Js)

	n := r.Group("/n/")
	n.GET(":organizationid/native.js", native.Js)
}
