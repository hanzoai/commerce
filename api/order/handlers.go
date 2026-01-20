package order

import (
	"github.com/gin-gonic/gin"

	checkoutApi "github.com/hanzoai/commerce/api/checkout"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(order.Order{})

	api.POST("/:orderid/authorize", publishedRequired, namespaced, checkoutApi.Authorize)
	api.POST("/:orderid/capture", publishedRequired, namespaced, checkoutApi.Capture)
	api.POST("/:orderid/charge", publishedRequired, namespaced, checkoutApi.Charge)

	api.POST("/:orderid/refund", adminRequired, namespaced, checkoutApi.Refund)
	api.GET("/:orderid/payments", adminRequired, namespaced, Payments)
	api.GET("/:orderid/returns", adminRequired, namespaced, Returns)
	api.GET("/:orderid/status", publishedRequired, namespaced, Status)

	api.GET("/:orderid/sendorderconfirmation", adminRequired, namespaced,SendOrderConfirmation)
	api.GET("/:orderid/sendrefundconfirmation",adminRequired, namespaced, SendRefundConfirmation)
	api.GET("/:orderid/sendfulfillmentconfirmation", adminRequired, namespaced,SendFulfillmentConfirmation)

	api.Create = Create
	api.Update = Update
	api.Patch = Patch
	api.Route(router, args...)
}
