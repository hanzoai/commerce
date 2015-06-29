package shipstation

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/thirdparty/shipstation/notify"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	group := router.Group("shipstation")

	group.POST("notify", notify.Post)
	group.GET("orders", orders.Get)
}
