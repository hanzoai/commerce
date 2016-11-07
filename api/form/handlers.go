package form

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"

	ml "crowdstart.com/cdn/mailinglist"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	group := router.Group("form")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:formid/submit", handleForm)
	group.POST("/:formid/subscribe", handleForm)
	group.GET("/:formid/js", ml.Js)

	// TODO: Remove deprecated endpoint
	tokenRequired := middleware.TokenRequired()
	api := rest.New(mailinglist.MailingList{})
	api.Route(router, tokenRequired)

	group = router.Group("mailinglist")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:mailinglistid/subscribe", handleForm)
	group.GET("/:mailinglistid/js", ml.Js)
}
