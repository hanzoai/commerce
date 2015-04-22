package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(mailinglist.MailingList{})

	api.POST("/:mailinglistid/subscribe", addSubscriber)
	api.GET("/:mailinglistid/js", js)

	api.Route(router, args...)
}
