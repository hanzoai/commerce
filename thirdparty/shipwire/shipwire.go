package shipwire

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("shipstation")

	basicAuth := middleware.BasicAuth()

	api.POST("/:organization", basicAuth, webhook.Process)
}
