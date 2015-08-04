package mailinglist

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"

	ml "crowdstart.com/cdn/mailinglist"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(mailinglist.MailingList{})

	group := router.Group("mailinglist")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:mailinglistid/subscribe", addSubscriber)
	group.GET("/:mailinglistid/js", ml.Js)

	api.Route(router, args...)
}
