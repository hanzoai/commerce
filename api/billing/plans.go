package billing

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/util/json/http"
)

//go:embed plans/subscription.json
var subscriptionJSON embed.FS

//go:embed plans/dns.json
var dnsJSON embed.FS

// canonicalPlan is the JSON shape from @hanzo/plans/*.json.
type canonicalPlan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceMonthly *float64 `json:"priceMonthly"` // dollars per month (null for custom)
	PriceAnnual  *float64 `json:"priceAnnual"`  // dollars per month billed annually (null for custom)
	Category     string   `json:"category"`
	Popular      bool     `json:"popular,omitempty"`
	ContactSales bool     `json:"contactSales,omitempty"`
	Features     []string `json:"features"`
	Limits       *struct {
		// Subscription (API) limits
		RequestsPerMinute *int `json:"requestsPerMinute,omitempty"`
		TokensPerMinute   *int `json:"tokensPerMinute,omitempty"`
		FreeCredit        *int `json:"freeCredit,omitempty"`
		MaxMembers        *int `json:"maxMembers,omitempty"`

		// DNS limits
		Zones          *int `json:"zones,omitempty"`
		RecordsPerZone *int `json:"recordsPerZone,omitempty"`
		QueriesPerDay  *int `json:"queriesPerDay,omitempty"`

		// World limits (hanzo.world / worldmonitor)
		MaxAlerts     *int `json:"maxAlerts,omitempty"`
		ApiRateLimit  *int `json:"apiRateLimit,omitempty"`
		McpRateLimit  *int `json:"mcpRateLimit,omitempty"`
	} `json:"limits,omitempty"`
	Payouts *struct {
		IdleResalePercent int    `json:"idleResalePercent"`
		Description       string `json:"description"`
	} `json:"payouts,omitempty"`
}

// staticPlan is the wire type returned by GET /billing/plans.
// Fields match the Plan type in the billing frontend's commerce-client.ts.
type staticPlan struct {
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	Price           int64    `json:"price"`         // monthly price in cents (0 = free)
	PriceAnnual     int64    `json:"priceAnnual"`   // annual price in cents per month
	Currency        string   `json:"currency"`
	Interval        string   `json:"interval"`
	IntervalCount   int      `json:"intervalCount"`
	TrialPeriodDays int      `json:"trialPeriodDays"`
	ContactSales    bool     `json:"contactSales,omitempty"`
	Popular         bool     `json:"popular,omitempty"`
	Features        []string `json:"features,omitempty"`
	Limits          *planLimits `json:"limits,omitempty"`
}

// planLimits captures per-plan quotas surfaced to clients.
type planLimits struct {
	// Subscription (API) limits
	RequestsPerMinute *int `json:"requestsPerMinute,omitempty"`
	TokensPerMinute   *int `json:"tokensPerMinute,omitempty"`
	FreeCredit        *int `json:"freeCredit,omitempty"`
	MaxMembers        *int `json:"maxMembers,omitempty"`

	// DNS limits
	Zones          *int `json:"zones,omitempty"`
	RecordsPerZone *int `json:"recordsPerZone,omitempty"`
	QueriesPerDay  *int `json:"queriesPerDay,omitempty"`

	// World limits (hanzo.world / worldmonitor)
	MaxAlerts    *int `json:"maxAlerts,omitempty"`
	ApiRateLimit *int `json:"apiRateLimit,omitempty"`
	McpRateLimit *int `json:"mcpRateLimit,omitempty"`
}

// hanzoPlans contains all plans loaded at init from embedded JSON files.
// Subscription plans have category "personal", "team", or "enterprise".
// DNS plans have category "dns".
var hanzoPlans []staticPlan

// dnsPlans is a filtered view containing only DNS plans for the /dns/plans endpoint.
var dnsPlans []staticPlan

func init() {
	hanzoPlans = loadPlansFromEmbed(subscriptionJSON, "plans/subscription.json")

	dns := loadPlansFromEmbed(dnsJSON, "plans/dns.json")
	dnsPlans = dns
	hanzoPlans = append(hanzoPlans, dns...)
}

// loadPlansFromEmbed reads an embedded JSON file and converts canonical plans
// to the staticPlan wire format. Panics on failure because plan data is required
// for the service to operate.
func loadPlansFromEmbed(fs embed.FS, path string) []staticPlan {
	data, err := fs.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("billing: failed to read embedded %s: %v", path, err))
	}

	var canonical []canonicalPlan
	if err := json.Unmarshal(data, &canonical); err != nil {
		panic(fmt.Sprintf("billing: failed to parse %s: %v", path, err))
	}

	plans := make([]staticPlan, len(canonical))
	for i, cp := range canonical {
		sp := staticPlan{
			Slug:          cp.ID,
			Name:          cp.Name,
			Description:   cp.Description,
			Category:      cp.Category,
			Currency:      "usd",
			Interval:      "monthly",
			IntervalCount: 1,
			ContactSales:  cp.ContactSales,
			Popular:       cp.Popular,
			Features:      cp.Features,
		}

		if cp.PriceMonthly != nil {
			sp.Price = int64(math.Round(*cp.PriceMonthly * 100))
		}
		if cp.PriceAnnual != nil {
			sp.PriceAnnual = int64(math.Round(*cp.PriceAnnual * 100))
		}

		if cp.Limits != nil {
			sp.Limits = &planLimits{
				RequestsPerMinute: cp.Limits.RequestsPerMinute,
				TokensPerMinute:   cp.Limits.TokensPerMinute,
				FreeCredit:        cp.Limits.FreeCredit,
				MaxMembers:        cp.Limits.MaxMembers,
				Zones:             cp.Limits.Zones,
				RecordsPerZone:    cp.Limits.RecordsPerZone,
				QueriesPerDay:     cp.Limits.QueriesPerDay,
				MaxAlerts:         cp.Limits.MaxAlerts,
				ApiRateLimit:      cp.Limits.ApiRateLimit,
				McpRateLimit:      cp.Limits.McpRateLimit,
			}
		}

		plans[i] = sp
	}

	return plans
}

// ListPlans returns the list of available plans, optionally filtered by category.
// Data is loaded at startup from embedded JSON plan definitions.
//
//	GET /api/v1/billing/plans
//	GET /api/v1/billing/plans?category=dns
func ListPlans(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		c.JSON(200, hanzoPlans)
		return
	}

	filtered := make([]staticPlan, 0)
	for _, p := range hanzoPlans {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	c.JSON(200, filtered)
}

// GetPlan returns a single plan by slug.
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

// lookupPlan finds a plan by slug across all loaded plans.
// Returns nil if not found.
func lookupPlan(slug string) *staticPlan {
	for i := range hanzoPlans {
		if hanzoPlans[i].Slug == slug {
			return &hanzoPlans[i]
		}
	}
	return nil
}

// SeedPlan is the public projection of a static plan for external consumers
// (e.g. the seed package) that cannot import unexported types.
type SeedPlan struct {
	Slug        string
	Name        string
	Description string
	Category    string
	PriceMonth  int64  // cents / month (0 for free)
	PriceYear   int64  // cents / month when billed annually (0 for free)
	Currency    string
}

// StaticPlans returns the embedded plan catalog as public SeedPlan values.
// Used by the seed package at bootstrap to sync Stripe products.
func StaticPlans() []SeedPlan {
	out := make([]SeedPlan, 0, len(hanzoPlans))
	for _, p := range hanzoPlans {
		out = append(out, SeedPlan{
			Slug:        p.Slug,
			Name:        p.Name,
			Description: p.Description,
			Category:    p.Category,
			PriceMonth:  p.Price,
			PriceYear:   p.PriceAnnual,
			Currency:    p.Currency,
		})
	}
	return out
}

// LookupStaticPlan returns the SeedPlan with the given slug, or nil.
// Safe for use by cmd/grant and other external callers.
func LookupStaticPlan(slug string) *SeedPlan {
	for _, p := range hanzoPlans {
		if p.Slug == slug {
			return &SeedPlan{
				Slug:        p.Slug,
				Name:        p.Name,
				Description: p.Description,
				Category:    p.Category,
				PriceMonth:  p.Price,
				PriceYear:   p.PriceAnnual,
				Currency:    p.Currency,
			}
		}
	}
	return nil
}
