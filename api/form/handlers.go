package form

import (
	"github.com/gin-gonic/gin"

	cdn "hanzo.io/api/cdn/form"
	"hanzo.io/middleware"
	"hanzo.io/models/form"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	rest.New(form.Form{}).Route(router, args...)

	f := router.Group("form")
	f.Use(middleware.AccessControl("*"))

	f.POST("/:formid/submit", handleForm)
	f.POST("/:formid/subscribe", handleForm)
	f.GET("/:formid/js", cdn.Js)

	// DEPRECATED
	m := router.Group("mailinglist")
	m.Use(middleware.AccessControl("*"))

	// m.POST("/:formid/submit", handleForm)
	// m.POST("/:formid/subscribe", handleForm)
	// m.GET("/:formid/js", cdn.Js)
}
