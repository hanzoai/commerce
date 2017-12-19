package order

import (
	"github.com/gin-gonic/gin"

	checkoutApi "hanzo.io/api/checkout"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(order.Order{})

	api.POST("/:orderid/authorize", publishedRequired, namespaced, checkoutApi.Authorize)
	api.POST("/:orderid/status", publishedRequired, namespaced, checkoutApi.Charge)
	api.POST("/:orderid/capture", publishedRequired, namespaced, checkoutApi.Capture)
	api.POST("/:orderid/charge", publishedRequired, namespaced, checkoutApi.Charge)

	api.POST("/:orderid/refund", adminRequired, namespaced, checkoutApi.Refund)
	api.GET("/:orderid/payments", adminRequired, namespaced, Payments)
	api.GET("/:orderid/returns", adminRequired, namespaced, Returns)

	api.Create = Create
	api.Update = Update
	api.Patch = Patch
	api.Route(router, args...)
}
