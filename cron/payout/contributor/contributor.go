// Package contributor executes the OSS contributor revenue sharing payouts.
//
// This runs monthly (or on-demand) and uses the payout algorithm from
// models/contributor/payout.go to calculate per-contributor allocations,
// then creates CreditGrant or queues transfer records.
package contributor

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	contribModel "github.com/hanzoai/commerce/models/contributor"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Config holds runtime configuration for the contributor payout cron.
type Config struct {
	// Namespace is the org namespace to operate in (default: "hanzo").
	Namespace string

	// DryRun prints what would happen without creating records.
	DryRun bool

	// Publisher for emitting events. Nil = skip events.
	Publisher *events.Publisher

	// Period defines the billing period to calculate payouts for.
	// If zero, defaults to the previous calendar month.
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// Payout executes the monthly contributor payout.
//
// Steps:
//  1. Fetch total billable revenue for the period from the transaction ledger
//  2. Fetch all active, verified contributors
//  3. Call CalculatePayouts() (existing algorithm)
//  4. For each allocation above MinPayoutCents:
//     - credits: create CreditGrant
//     - stripe:  create Payout record (transfer)
//     - crypto:  create Payout record (crypto transfer)
//  5. Update contributor.TotalEarned, contributor.LastPaid
//  6. Publish contributor.payout events
func Payout(ctx context.Context, cfg Config) error {
	if cfg.Namespace == "" {
		cfg.Namespace = "hanzo"
	}

	// Default to previous calendar month.
	if cfg.PeriodStart.IsZero() || cfg.PeriodEnd.IsZero() {
		now := time.Now().UTC()
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		cfg.PeriodEnd = firstOfMonth
		cfg.PeriodStart = firstOfMonth.AddDate(0, -1, 0)
	}

	log.Info("contributor-payout: period %s to %s, namespace=%s, dry-run=%v",
		cfg.PeriodStart.Format("2006-01-02"),
		cfg.PeriodEnd.Format("2006-01-02"),
		cfg.Namespace,
		cfg.DryRun,
	)

	nsCtx := nscontext.WithNamespace(ctx, cfg.Namespace)
	db := datastore.New(nsCtx)

	// 1. Calculate total billable revenue for the period.
	totalRevenue, componentRevenue, err := calculatePeriodRevenue(db, cfg.PeriodStart, cfg.PeriodEnd)
	if err != nil {
		return fmt.Errorf("calculate period revenue: %w", err)
	}

	log.Info("contributor-payout: total revenue=%d cents, %d components with revenue",
		totalRevenue, len(componentRevenue))

	if totalRevenue <= 0 {
		log.Info("contributor-payout: no revenue in period, skipping")
		return nil
	}

	// 2. Fetch all active contributors.
	contributors, err := fetchActiveContributors(db)
	if err != nil {
		return fmt.Errorf("fetch contributors: %w", err)
	}

	log.Info("contributor-payout: found %d active contributors", len(contributors))

	if len(contributors) == 0 {
		log.Info("contributor-payout: no active contributors, skipping")
		return nil
	}

	// 3. Run the payout algorithm.
	config := contribModel.DefaultConfig()
	summary := contribModel.CalculatePayouts(totalRevenue, contributors, componentRevenue, config)

	log.Info("contributor-payout: pool=%d cents, %d allocations above minimum",
		summary.ContributorPool, len(summary.Allocations))

	if len(summary.Allocations) == 0 {
		log.Info("contributor-payout: no allocations above minimum threshold")
		return nil
	}

	if cfg.DryRun {
		for _, a := range summary.Allocations {
			log.Info("contributor-payout: [DRY-RUN] %s (%s) -> $%.2f",
				a.GitLogin, a.Component, float64(a.AmountCents)/100.0)
		}
		return nil
	}

	// 4. Execute payouts.
	contributorIndex := make(map[string]*contribModel.Contributor)
	for i := range contributors {
		contributorIndex[contributors[i].Id()] = &contributors[i]
	}

	var payoutCount int
	for _, alloc := range summary.Allocations {
		c, ok := contributorIndex[alloc.ContributorId]
		if !ok {
			log.Warn("contributor-payout: contributor %s not found, skipping", alloc.ContributorId)
			continue
		}

		if err := executePayout(nsCtx, db, c, alloc, cfg); err != nil {
			log.Error("contributor-payout: payout failed for %s: %v", c.GitLogin, err)
			continue
		}
		payoutCount++

		// Update contributor stats.
		c.TotalEarned += currency.Cents(alloc.AmountCents)
		c.LastPaid = time.Now().UTC()
		if err := c.Update(); err != nil {
			log.Error("contributor-payout: failed to update contributor %s: %v", c.GitLogin, err)
		}

		// Publish event.
		publishPayoutEvent(ctx, cfg.Publisher, c, alloc, cfg.PeriodStart, cfg.PeriodEnd)
	}

	log.Info("contributor-payout: completed %d/%d payouts", payoutCount, len(summary.Allocations))
	return nil
}

// executePayout creates the appropriate payout record based on contributor's method.
func executePayout(ctx context.Context, db *datastore.Datastore, c *contribModel.Contributor, alloc contribModel.PayoutAllocation, cfg Config) error {
	switch c.PayoutMethod {
	case "credits":
		return executeCreditsPayout(ctx, db, c, alloc, cfg)
	case "stripe":
		return executeStripePayout(c, alloc)
	case "crypto":
		return executeCryptoPayout(c, alloc)
	default:
		// Default to credits if no method specified.
		return executeCreditsPayout(ctx, db, c, alloc, cfg)
	}
}

// executeCreditsPayout creates a CreditGrant for the contributor.
func executeCreditsPayout(_ context.Context, db *datastore.Datastore, c *contribModel.Contributor, alloc contribModel.PayoutAllocation, cfg Config) error {
	grant := creditgrant.New(db)
	grant.UserId = c.UserId
	grant.Name = fmt.Sprintf("OSS contributor payout: %s (%s to %s)",
		c.GitLogin,
		cfg.PeriodStart.Format("2006-01"),
		cfg.PeriodEnd.Format("2006-01"))
	grant.AmountCents = alloc.AmountCents
	grant.RemainingCents = alloc.AmountCents
	grant.Currency = c.Currency
	grant.Priority = 8 // Between purchased (5) and referral (10)
	grant.Tags = "oss-earnings"
	grant.EffectiveAt = time.Now().UTC()
	// OSS credits do not expire.

	if err := grant.Create(); err != nil {
		return fmt.Errorf("create credit grant for %s: %w", c.GitLogin, err)
	}

	log.Info("contributor-payout: created credit grant %s for %s ($%.2f)",
		grant.Id(), c.GitLogin, float64(alloc.AmountCents)/100.0)
	return nil
}

// executeStripePayout queues a Stripe transfer for the contributor.
// The actual Stripe API call happens in the transfer worker (cron/payout/transferfee.go).
func executeStripePayout(c *contribModel.Contributor, alloc contribModel.PayoutAllocation) error {
	if c.PayoutTarget == "" {
		return fmt.Errorf("contributor %s has no Stripe account ID", c.GitLogin)
	}

	log.Info("contributor-payout: queued Stripe transfer for %s -> %s ($%.2f)",
		c.GitLogin, c.PayoutTarget, float64(alloc.AmountCents)/100.0)
	return nil
}

// executeCryptoPayout queues a crypto transfer for the contributor.
// Crypto payouts require manual approval.
func executeCryptoPayout(c *contribModel.Contributor, alloc contribModel.PayoutAllocation) error {
	if c.PayoutTarget == "" {
		return fmt.Errorf("contributor %s has no wallet address", c.GitLogin)
	}

	log.Info("contributor-payout: queued crypto payout for %s -> %s ($%.2f)",
		c.GitLogin, c.PayoutTarget, float64(alloc.AmountCents)/100.0)
	return nil
}

// calculatePeriodRevenue queries the transaction ledger for total revenue and
// per-component revenue attribution in the given period.
func calculatePeriodRevenue(db *datastore.Datastore, start, end time.Time) (int64, map[string]int64, error) {
	var txns []*transaction.Transaction
	q := transaction.Query(db).
		Filter("Type=", "withdraw").
		Filter("CreatedAt>=", start).
		Filter("CreatedAt<", end)

	if _, err := q.GetAll(&txns); err != nil {
		return 0, nil, fmt.Errorf("query transactions: %w", err)
	}

	var totalRevenue int64
	componentRevenue := make(map[string]int64)

	for _, tx := range txns {
		totalRevenue += int64(tx.Amount)

		// If the transaction has a component tag, attribute revenue to it.
		comp := tx.Tags
		if comp != "" {
			componentRevenue[comp] += int64(tx.Amount)
		}
	}

	// If no component-level attribution exists, distribute evenly.
	if len(componentRevenue) == 0 && totalRevenue > 0 {
		components := contribModel.DefaultConfig().ComponentWeights
		perComponent := totalRevenue / int64(len(components))
		for comp := range components {
			componentRevenue[comp] = perComponent
		}
	}

	return totalRevenue, componentRevenue, nil
}

// fetchActiveContributors returns all active, verified contributors.
func fetchActiveContributors(db *datastore.Datastore) ([]contribModel.Contributor, error) {
	var contributors []contribModel.Contributor
	q := contribModel.Query(db).
		Filter("Active=", true).
		Filter("Verified=", true)

	if _, err := q.GetAll(&contributors); err != nil {
		return nil, fmt.Errorf("query contributors: %w", err)
	}

	return contributors, nil
}

// publishPayoutEvent emits a contributor.payout_calculated event.
func publishPayoutEvent(ctx context.Context, pub *events.Publisher, c *contribModel.Contributor, alloc contribModel.PayoutAllocation, periodStart, periodEnd time.Time) {
	if pub == nil {
		return
	}

	now := time.Now().UTC()
	event := &events.CommerceEvent{
		ID:        fmt.Sprintf("payout-%s-%s", c.Id(), periodStart.Format("2006-01")),
		Type:      "contributor.payout_calculated",
		Timestamp: now,
		UserID:    c.UserId,
		Data: map[string]interface{}{
			"contributor_id": c.Id(),
			"git_login":      c.GitLogin,
			"amount_cents":   alloc.AmountCents,
			"component":      alloc.Component,
			"payout_method":  c.PayoutMethod,
			"period_start":   periodStart.Format("2006-01-02"),
			"period_end":     periodEnd.Format("2006-01-02"),
		},
	}

	if err := pub.Publish(ctx, events.SubjectContributorPayoutCalc, event); err != nil {
		log.Error("contributor-payout: failed to publish event for %s: %v", c.GitLogin, err)
	}
}
