package periodic

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"

	"crowdstart.com/periodic/affiliate_transfer"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("periodic")

	api.GET("/affiliate_transfer/", adminRequired, affiliate_transfer.Run)
}
