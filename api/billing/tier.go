package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetTier returns the billing tier, limits, and effective balance for a user.
//
// For IAM-authenticated requests the tier is read from the JWT claim.
// For service-to-service calls the tier may be passed as a query parameter.
//
//	GET /api/v1/billing/tier?user=hanzo/alice
//
// Response includes the tier config plus the effective available balance
// (which for free-tier users includes the daily replenishing credit).
func GetTier(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	// Resolve tier: prefer IAM claim, fall back to query param, default to free.
	tierName := tier.Free
	if iamTier := iammiddleware.GetIAMTier(c); iamTier != "" {
		tierName = tier.Parse(iamTier)
	} else if qTier := c.Query("tier"); qTier != "" {
		tierName = tier.Parse(qTier)
	}

	cfg := tier.Get(tierName)

	// Fetch the user's prepaid balance.
	cur := currency.Type("usd")
	datas, err := txutil.GetTransactionsByCurrency(ctx, user, "iam-user", cur, !org.Live)
	if err != nil {
		http.Fail(c, 500, "failed to query balance", err)
		return
	}

	var prepaidBalance, holds currency.Cents
	if data, ok := datas.Data[cur]; ok {
		prepaidBalance = data.Balance
		holds = data.Holds
	}

	prepaidAvailable := prepaidBalance - holds
	if prepaidAvailable < 0 {
		prepaidAvailable = 0
	}

	// For free-tier users, compute the daily replenishing credit.
	// The daily credit resets at midnight UTC and does not accumulate.
	var dailyRemaining int64
	if cfg.HasDailyCredits() {
		dailyUsed := dailyUsageCents(ctx, user, !org.Live)
		dailyRemaining = cfg.DailyCreditsCents - dailyUsed
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
	}

	effectiveAvailable := int64(prepaidAvailable) + dailyRemaining

	c.JSON(200, gin.H{
		"user": user,
		"tier": gin.H{
			"name":              cfg.Name,
			"displayName":       cfg.DisplayName,
			"maxAgents":         cfg.MaxAgents,
			"unlimitedAgents":   cfg.IsUnlimitedAgents(),
			"dailyCreditsCents": cfg.DailyCreditsCents,
			"allowedModels":     cfg.AllowedModels,
		},
		"balance": gin.H{
			"currency":           cur,
			"prepaidAvailable":   prepaidAvailable,
			"dailyRemaining":     dailyRemaining,
			"effectiveAvailable": effectiveAvailable,
		},
	})
}

// dailyUsageCents sums the api-usage withdrawals for a user since
// midnight UTC today. This determines how much of the free-tier daily
// credit has been consumed.
func dailyUsageCents(ctx context.Context, user string, isTest bool) int64 {
	db := datastore.New(ctx)
	rootKey := db.NewKey("synckey", "", 1, nil)

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", isTest).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", user).
		Filter("Tags=", "api-usage")

	if _, err := q.GetAll(&transs); err != nil {
		return 0
	}

	var total int64
	for _, t := range transs {
		if !t.CreatedAt.Before(todayStart) {
			total += int64(t.Amount)
		}
	}

	return total
}
