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
