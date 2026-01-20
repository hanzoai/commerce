package api

import (
	"github.com/gin-gonic/gin"
	"github.com/hanzoai/commerce/util/router"
)

// Wire up stripe endpoint
func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("ethereum")
	api.POST("/webhook", Webhook)
}
