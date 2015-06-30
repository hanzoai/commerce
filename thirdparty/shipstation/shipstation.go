package shipstation

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/shipstation/notify"
	"crowdstart.com/thirdparty/shipstation/order"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("shipstation")

	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api.POST("notify", publishedRequired, notify.Post)
	api.GET("order", publishedRequired, order.Get)
}
