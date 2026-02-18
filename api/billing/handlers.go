package billing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

// Route registers billing endpoints for service-to-service calls.
// These are internal endpoints used by Cloud-API; require admin token.
func Route(r router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("billing")
	api.Use(adminRequired)

	api.GET("/balance", GetBalance)
	api.GET("/balance/all", GetBalanceAll)
	api.GET("/usage", GetUsage)
	api.POST("/usage", RecordUsage)
}
