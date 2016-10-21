package api

import (
	"crowdstart.com/util/router"
	"github.com/gin-gonic/gin"
)

// Wire up stripe endpoint
func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("stripe")
	api.POST("/webhook", Webhook)
	api.POST("/callback", Callback)
}
