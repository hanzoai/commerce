package costs

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

// Route registers the vendor-cost (COGS) endpoints. These are admin-only,
// service-to-service reads consumed by the admin business board — gated by the
// SAME token gate the billing admin uses (middleware.TokenRequired(permission.Admin)),
// so a browser can never reach them and only a global-admin-gated proxy forwards.
func Route(r router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("costs")
	api.Use(adminRequired)

	api.GET("", GetCosts)
	api.GET("/margin", GetMargin)
}

// GetCosts returns the per-vendor COGS breakdown for a period.
//
//	GET /v1/costs?period=YYYY-MM
//
// Response: {period, vendors:[{vendor,service,amountCents,period,source,note}], totalCents, currency}.
func GetCosts(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	p := period(c.Query("period"))

	report, _ := buildReport(ctx, org.TestMode(), p)
	c.JSON(200, report)
}

// GetMargin returns revenue (the usage ledger) minus COGS (this package) plus the
// gross-margin percentage — the MARGIN view for admin.hanzo.ai.
//
//	GET /v1/costs/margin?period=YYYY-MM
//
// Revenue is the sum of api-usage charges in the period (what customers paid);
// COGS is the vendor total. Kept DRY with GetCosts: buildReport returns both the
// vendor lines and the ledger aggregate, so revenue and cost come from one walk.
func GetMargin(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	p := period(c.Query("period"))

	report, ledger := buildReport(ctx, org.TestMode(), p)

	revenue := ledger.RevenueCents
	cogs := report.TotalCents
	margin := revenue - cogs

	c.JSON(200, MarginReport{
		Period:         p,
		RevenueCents:   revenue,
		COGSCents:      cogs,
		MarginCents:    margin,
		GrossMarginPct: grossMarginPct(revenue, margin),
		Vendors:        report.Vendors,
		Currency:       "usd",
	})
}
