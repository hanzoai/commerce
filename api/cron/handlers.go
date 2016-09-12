package cron

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"

	"crowdstart.com/cron/affiliate"
	"crowdstart.com/cron/platform"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("cron")

	api.GET("/affiliate/payout", adminRequired, affiliate.Payout)
	api.GET("/platform/payout", adminRequired, platform.Payout)
}
