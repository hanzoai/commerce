package form

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/mailinglist"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"

	ml "hanzo.io/api/cdn/mailinglist"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	rest.New(mailinglist.MailingList{}).Route(router, args...)

	group := router.Group("form")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:formid/submit", handleForm)
	group.POST("/:formid/subscribe", handleForm)
	group.GET("/:formid/js", ml.Js)

	group = router.Group("mailinglist")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:mailinglistid/subscribe", handleForm)
	group.GET("/:mailinglistid/js", ml.Js)
}
