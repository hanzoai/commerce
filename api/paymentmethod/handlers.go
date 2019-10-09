package paymentmethod

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	accountRequired := middleware.AccountRequired()
	namespaced := middleware.Namespace()

	api := router.Group("paymentmethod", args...)
	api.POST("/:paymentmethodtype", accountRequired, namespaced, create)
}
