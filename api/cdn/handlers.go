package cdn

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/cdn/analytics"
	"github.com/hanzoai/commerce/api/cdn/native"
)

func Route(r *zip.Group, args ...zip.Handler) {
	a := r.Group("/a/")
	a.Raw(http.MethodGet, ":organizationid", analytics.Js)
	a.Raw(http.MethodGet, ":organizationid/analytics.js", analytics.Js)
	a.Raw(http.MethodGet, ":organizationid/js", analytics.Js)

	n := r.Group("/n/")
	n.Raw(http.MethodGet, ":organizationid/native.js", native.Js)
}
