package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models/mailinglist"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(mailinglist.MailingList{})

	group := router.Group("mailinglist")
	group.Use(middleware.AccessControl("*"))

	group.POST("/:mailinglistid/subscribe", addSubscriber)
	group.GET("/:mailinglistid/js", js)

	api.Route(router, args...)
}
