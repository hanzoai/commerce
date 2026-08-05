package form

import (
	"github.com/zap-proto/zip"

	cdn "github.com/hanzoai/commerce/api/cdn/form"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	rest.New(form.Form{}).Route(router, args...)

	f := router.Group("form")
	f.Use(middleware.AccessControl("*"))

	f.Post("/:formid/submit", handleForm)
	f.Post("/:formid/subscribe", handleForm)
	f.Get("/:formid/js", cdn.Js)

	// DEPRECATED
	m := router.Group("mailinglist")
	m.Use(middleware.AccessControl("*"))

	m.Post("/:formid/submit", handleForm)
	m.Post("/:formid/subscribe", handleForm)
	m.Get("/:formid/js", cdn.Js)
}
