package cdn

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/cdn/analytics"
	"github.com/hanzoai/commerce/api/cdn/form"
	"github.com/hanzoai/commerce/api/cdn/native"
)

func Route(r *zip.Group, args ...zip.Handler) {
	a := r.Group("/a/")
	a.Raw(http.MethodGet, ":organizationid", analytics.Js)
	a.Raw(http.MethodGet, ":organizationid/analytics.js", analytics.Js)
	a.Raw(http.MethodGet, ":organizationid/js", analytics.Js)

	f := r.Group("/f/")
	f.Raw(http.MethodGet, ":formid/form.js", form.Js)
	f.Raw(http.MethodGet, ":formid/js", form.Js)

	// DEPRECATED
	m := r.Group("/m/")
	m.Raw(http.MethodGet, ":formid/mailinglist.js", form.Js)
	m.Raw(http.MethodGet, ":formid/js", form.Js)

	n := r.Group("/n/")
	n.Raw(http.MethodGet, ":organizationid/native.js", native.Js)
}
