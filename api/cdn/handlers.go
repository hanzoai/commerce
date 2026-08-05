package cdn

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/cdn/analytics"
	"github.com/hanzoai/commerce/api/cdn/form"
	"github.com/hanzoai/commerce/api/cdn/native"
)

func Route(r zip.Router, args ...zip.Handler) {
	a := r.Group("/a/")
	a.Get(":organizationid", analytics.Js)
	a.Get(":organizationid/analytics.js", analytics.Js)
	a.Get(":organizationid/js", analytics.Js)

	f := r.Group("/f/")
	f.Get(":formid/form.js", form.Js)
	f.Get(":formid/js", form.Js)

	// DEPRECATED
	m := r.Group("/m/")
	m.Get(":formid/mailinglist.js", form.Js)
	m.Get(":formid/js", form.Js)

	n := r.Group("/n/")
	n.Get(":organizationid/native.js", native.Js)
}
