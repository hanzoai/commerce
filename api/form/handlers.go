package form

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/form"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(form.Form{})

	api.POST("/:formid/submit", handleForm)
	api.POST("/:formid/subscribe", handleForm)
	api.GET("/:formid/js", formJs)

	// TODO: Remove deprecated endpoints
	group := router.Group("mailinglist")
	group.POST("/:mailinglistid/subscribe", handleForm)
	group.GET("/:mailinglistid/js", formJs)
	router.GET("/m/:mailinglistid/mailinglist.js", formJs)
}
