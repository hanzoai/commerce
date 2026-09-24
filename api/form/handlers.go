package form

import (
	"net/http"

	"github.com/zap-proto/zip"

	cdn "github.com/hanzoai/commerce/api/cdn/form"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router *zip.Group, args ...zip.Handler) {
	rest.New(form.Form{}).Route(router, args...)

	f := router.Group("form")
	f.Use(middleware.AccessControl("*"))

	f.Raw(http.MethodPost, "/:formid/submit", handleForm)
	f.Raw(http.MethodPost, "/:formid/subscribe", handleForm)
	f.Raw(http.MethodGet, "/:formid/js", cdn.Js)

	// DEPRECATED
	m := router.Group("mailinglist")
	m.Use(middleware.AccessControl("*"))

	m.Raw(http.MethodPost, "/:formid/submit", handleForm)
	m.Raw(http.MethodPost, "/:formid/subscribe", handleForm)
	m.Raw(http.MethodGet, "/:formid/js", cdn.Js)
}
