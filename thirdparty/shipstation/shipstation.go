package shipstation

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/shipstation/export"
	"crowdstart.com/thirdparty/shipstation/shipnotify"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("shipstation")

	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	adminRequired := middleware.TokenRequired(permission.Admin)

	api.GET("", adminRequired, export.Export)
	api.POST("", adminRequired, shipnotify.ShipNotify)
}
