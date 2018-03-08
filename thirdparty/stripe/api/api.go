package api

import (
	"hanzo.io/util/router"
	"github.com/gin-gonic/gin"
)

// Wire up stripe endpoint
func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("stripe")
	api.GET("/callback", Callback)
	api.POST("/webhook", Webhook)
}
