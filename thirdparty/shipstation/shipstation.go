package shipstation

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/thirdparty/shipstation/notify"
	"crowdstart.com/thirdparty/shipstation/order"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	group := router.Group("shipstation")

	group.POST("notify", notify.Post)
	group.GET("order", order.Get)
}
