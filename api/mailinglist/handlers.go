package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(mailinglist.MailingList{})

	api.POST("/:mailinlistid/subscribe", publishedRequired, namespaced, addSubscriber)

	api.Route(router, args...)
}
