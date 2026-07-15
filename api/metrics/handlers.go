package metrics

import (
	"strconv"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

// Route registers the SaaS-operations god-view. Like api/costs this is a PLATFORM
// aggregate, so on top of the route-level token gate the handler ALSO enforces
// middleware.RequirePlatformAdmin — the route-level TokenRequired(permission.Admin)
// is a NO-OP on the IAM path (it admits any IAM-authenticated request without
// checking the Admin bit), so the in-handler gate is the real boundary.
//
// External (gateway) path: GET /v1/commerce/metrics/saas. Internal (this bundle):
// GET /v1/metrics/saas — where the cloud admin proxy and the console's
// global-admin-gated commerce proxy reach it with the service token.
func Route(r zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("metrics")
	api.Use(adminRequired)

	api.Get("/saas", GetSaaS)
}

// GetSaaS returns the whole-business SaaS snapshot — revenue (MRR/ARR + new/churn),
// subscription mix, metered usage / LLM-obs, and top customers — from ONE cross-org
// walk of commerce's subscription + usage ledgers.
//
//	GET /v1/commerce/metrics/saas?window=30d&limit=20
//
// window: 7d | 30d | 90d | mtd | all (default 30d) — bounds usage + new/churn.
// limit:  cap for the top-orgs / customers lists (default 20, max 200).
func GetSaaS(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	return c.JSON(200, rollup(options{
		window: c.Query("window"),
		limit:  parseLimit(c.Query("limit")),
		test:   c.Query("test") == "true",
	}))
}

// parseLimit clamps the top-N cap to [1,200], defaulting to defaultLimit.
func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultLimit
	}
	if n > 200 {
		return 200
	}
	return n
}
