package costs

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

// Route registers the vendor-cost (COGS) endpoints. These are a PLATFORM
// god-view (what WE pay our vendors across the whole business), so on top of the
// route-level token gate every handler ALSO enforces requireCostsAdmin — see its
// doc for why the middleware alone is not enough.
func Route(r router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("costs")
	api.Use(adminRequired)

	api.GET("", GetCosts)
	api.GET("/margin", GetMargin)
}

// requireCostsAdmin is the in-handler gate for the vendor-cost god-view. It fails
// closed (403) unless the caller is a PLATFORM admin, and is enforced INSIDE each
// handler because the route-level middleware.TokenRequired(permission.Admin) is a
// NO-OP on the IAM path: it short-circuits (c.Next) for ANY IAM-authenticated
// request WITHOUT checking the Admin bit, and IAMTokenRequired stamps
// iam_authenticated=true whenever an X-Org-Id header is present — which the gateway
// sets on EVERY authenticated call. A handler must never trust that gate on its own.
//
// These figures are cross-tenant PLATFORM spend/margin, so this gates on the
// STRICTER GlobalAdmin predicate (or the trusted internal service token), NEVER the
// org-level Admin bit an org owner carries. That gate is shared with the SaaS
// metrics god-view (api/metrics) as the single middleware.RequirePlatformAdmin
// predicate — one and only one definition of "may read cross-org platform data",
// so costs and metrics can never drift apart. See middleware.MayReadPlatform for
// the exact principals admitted and why org-level admin is refused.
func requireCostsAdmin(c *gin.Context) bool {
	return middleware.RequirePlatformAdmin(c)
}

// GetCosts returns the per-vendor COGS breakdown for a period.
//
//	GET /v1/costs?period=YYYY-MM
//
// Response: {period, vendors:[{vendor,service,amountCents,period,source,note}], totalCents, currency}.
func GetCosts(c *gin.Context) {
	if !requireCostsAdmin(c) {
		return
	}
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
	if !requireCostsAdmin(c) {
		return
	}
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
