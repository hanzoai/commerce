package periodic

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"

	"crowdstart.com/periodic/stripe_payout"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("periodic")

	api.GET("/stripe_payout/", adminRequired, stripe_payout.Run)
}
