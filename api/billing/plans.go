package billing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/util/json/http"
)

// staticPlan is the wire type returned by GET /billing/plans.
// Fields match the Plan type in the billing frontend's commerce-client.ts.
type staticPlan struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Price           int64  `json:"price"`    // monthly price in cents (0 = free)
	PriceAnnual     int64  `json:"priceAnnual"` // annual price in cents
	Currency        string `json:"currency"`
	Interval        string `json:"interval"`
	IntervalCount   int    `json:"intervalCount"`
	TrialPeriodDays int    `json:"trialPeriodDays"`
	ContactSales    bool   `json:"contactSales,omitempty"`
}

// hanzoPlans is the canonical list of Hanzo subscription plans.
// Prices are in USD cents per month. Source: /pricing/plans/subscription.json.
var hanzoPlans = []staticPlan{
	{
		Slug:          "developer",
		Name:          "Developer",
		Description:   "Get started for free. Explore the API with generous included credits.",
		Price:         0,
		PriceAnnual:   0,
		Currency:      "usd",
		Interval:      "monthly",
		IntervalCount: 1,
	},
	{
		Slug:          "pro",
		Name:          "Pro",
		Description:   "For developers shipping real products. Higher limits and priority support.",
		Price:         4900,  // $49/mo
		PriceAnnual:   3900,  // $39/mo billed annually
		Currency:      "usd",
		Interval:      "monthly",
		IntervalCount: 1,
	},
	{
		Slug:          "team",
		Name:          "Team",
		Description:   "For teams building together. SSO, shared billing, and custom training.",
		Price:         19900, // $199/mo
		PriceAnnual:   15900, // $159/mo billed annually
		Currency:      "usd",
		Interval:      "monthly",
		IntervalCount: 1,
	},
	{
		Slug:          "enterprise",
		Name:          "Enterprise",
		Description:   "Full-scale AI infrastructure. Dedicated support, SLA, and on-prem deployment.",
		Price:         999900, // $9999/mo
		PriceAnnual:   799900, // $7999/mo billed annually
		Currency:      "usd",
		Interval:      "monthly",
		IntervalCount: 1,
	},
	{
		Slug:         "custom",
		Name:         "Custom",
		Description:  "Need more? We'll build a plan around your infrastructure, compliance, and scale requirements.",
		Currency:     "usd",
		ContactSales: true,
	},
}

// ListPlans returns the list of available subscription plans.
//
//	GET /api/v1/billing/plans
func ListPlans(c *gin.Context) {
	c.JSON(200, hanzoPlans)
}

// GetPlan returns a single plan by slug or index.
//
//	GET /api/v1/billing/plans/:id
func GetPlan(c *gin.Context) {
	id := c.Param("id")
	for _, p := range hanzoPlans {
		if p.Slug == id {
			c.JSON(200, p)
			return
		}
	}
	http.Fail(c, 404, "plan not found", nil)
}
