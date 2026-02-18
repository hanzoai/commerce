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

	// Balance & usage (existing)
	api.GET("/balance", GetBalance)
	api.GET("/balance/all", GetBalanceAll)
	api.GET("/usage", GetUsage)
	api.POST("/usage", RecordUsage)
	api.POST("/deposit", Deposit)
	api.POST("/credit", GrantStarterCredit)
	api.POST("/refund", Refund)

	// Meters
	api.POST("/meters", CreateMeter)
	api.GET("/meters", ListMeters)
	api.GET("/meters/:id", GetMeter)

	// Meter events
	api.POST("/meter-events", RecordMeterEvents)
	api.GET("/meter-events/summary", GetMeterEventsSummary)

	// Credit grants
	api.POST("/credit-grants", CreateCreditGrant)
	api.GET("/credit-grants", ListCreditGrants)
	api.GET("/credit-balance", GetCreditBalance)
	api.POST("/credit-grants/:id/void", VoidCreditGrant)

	// Pricing rules
	api.POST("/pricing-rules", CreatePricingRule)
	api.GET("/pricing-rules", ListPricingRules)
	api.DELETE("/pricing-rules/:id", DeletePricingRule)

	// Invoice preview
	api.POST("/invoice-preview", InvoicePreview)

	// ZAP protocol endpoint
	api.POST("/zap", ZapDispatch)
}
