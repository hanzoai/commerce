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

// canonicalPlan is the JSON shape from @hanzo/plans/subscription.json.
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
		RequestsPerMinute *int `json:"requestsPerMinute"`
		TokensPerMinute   *int `json:"tokensPerMinute"`
		FreeCredit        *int `json:"freeCredit,omitempty"`
		MaxMembers        *int `json:"maxMembers,omitempty"`
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
	Price           int64    `json:"price"`         // monthly price in cents (0 = free)
	PriceAnnual     int64    `json:"priceAnnual"`   // annual price in cents per month
	Currency        string   `json:"currency"`
	Interval        string   `json:"interval"`
	IntervalCount   int      `json:"intervalCount"`
	TrialPeriodDays int      `json:"trialPeriodDays"`
	ContactSales    bool     `json:"contactSales,omitempty"`
	Popular         bool     `json:"popular,omitempty"`
	Features        []string `json:"features,omitempty"`
	Limits          *struct {
		RequestsPerMinute *int `json:"requestsPerMinute,omitempty"`
		TokensPerMinute   *int `json:"tokensPerMinute,omitempty"`
		FreeCredit        *int `json:"freeCredit,omitempty"`
		MaxMembers        *int `json:"maxMembers,omitempty"`
	} `json:"limits,omitempty"`
}

// hanzoPlans is loaded at init from the embedded @hanzo/plans/subscription.json.
var hanzoPlans []staticPlan

func init() {
	data, err := subscriptionJSON.ReadFile("plans/subscription.json")
	if err != nil {
		panic(fmt.Sprintf("billing: failed to read embedded subscription.json: %v", err))
	}

	var canonical []canonicalPlan
	if err := json.Unmarshal(data, &canonical); err != nil {
		panic(fmt.Sprintf("billing: failed to parse subscription.json: %v", err))
	}

	hanzoPlans = make([]staticPlan, len(canonical))
	for i, cp := range canonical {
		sp := staticPlan{
			Slug:          cp.ID,
			Name:          cp.Name,
			Description:   cp.Description,
			Currency:      "usd",
			Interval:      "monthly",
			IntervalCount: 1,
			ContactSales:  cp.ContactSales,
			Popular:       cp.Popular,
			Features:      cp.Features,
			Limits:        nil,
		}
		// Convert dollar prices to cents.
		if cp.PriceMonthly != nil {
			sp.Price = int64(math.Round(*cp.PriceMonthly * 100))
		}
		if cp.PriceAnnual != nil {
			sp.PriceAnnual = int64(math.Round(*cp.PriceAnnual * 100))
		}
		// Copy limits.
		if cp.Limits != nil {
			sp.Limits = &struct {
				RequestsPerMinute *int `json:"requestsPerMinute,omitempty"`
				TokensPerMinute   *int `json:"tokensPerMinute,omitempty"`
				FreeCredit        *int `json:"freeCredit,omitempty"`
				MaxMembers        *int `json:"maxMembers,omitempty"`
			}{
				RequestsPerMinute: cp.Limits.RequestsPerMinute,
				TokensPerMinute:   cp.Limits.TokensPerMinute,
				FreeCredit:        cp.Limits.FreeCredit,
				MaxMembers:        cp.Limits.MaxMembers,
			}
		}
		hanzoPlans[i] = sp
	}
}

// ListPlans returns the list of available subscription plans.
// Data is loaded at startup from @hanzo/plans/subscription.json (embedded).
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
