// Package contributor ACCRUES the OSS contributor revenue share. It runs the
// payout algorithm from models/contributor/payout.go and records each allocation
// as a fee.Fee — a tracked payable — and nothing more.
//
// It executes no payout. It used to dispatch on Contributor.PayoutMethod into
// three executors: a CreditGrant mint, a Stripe no-op that reported success for
// money that never moved, and an on-chain ERC-20 transfer signed by the treasury
// key. Payment is a human decision made out-of-band and recorded afterwards.
package contributor

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	contribModel "github.com/hanzoai/commerce/models/contributor"
	"github.com/hanzoai/commerce/models/fee"
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

// Payout accrues the monthly contributor revenue share.
//
// Steps:
//  1. Fetch total billable revenue for the period from the transaction ledger
//  2. Fetch all active, verified contributors
//  3. Call CalculatePayouts() (existing algorithm)
//  4. Accrue a payable (fee.Fee) for each allocation above MinPayoutCents
//  5. Update contributor.TotalPending (what we owe)
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

	// 4. Accrue payables.
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

		if err := accrue(db, c, alloc); err != nil {
			log.Error("contributor-payout: accrual failed for %s: %v", c.GitLogin, err)
			continue
		}
		payoutCount++

		// TotalPending is what we OWE this contributor. TotalEarned/LastPaid are
		// paid-to-date and move only when a human records a payment.
		c.TotalPending += currency.Cents(alloc.AmountCents)
		if err := c.Update(); err != nil {
			log.Error("contributor-payout: failed to update contributor %s: %v", c.GitLogin, err)
		}

		publishPayoutEvent(ctx, cfg.Publisher, c, alloc, "", cfg.PeriodStart, cfg.PeriodEnd)
	}

	log.Info("contributor-payout: accrued %d/%d allocations", payoutCount, len(summary.Allocations))
	return nil
}

// accrue records what we OWE this contributor as a fee.Fee — a tracked payable.
// It moves no value. A human pays out-of-band and records it against the fee.
func accrue(db *datastore.Datastore, c *contribModel.Contributor, alloc contribModel.PayoutAllocation) error {
	fe := fee.New(db)
	fe.Name = fmt.Sprintf("OSS contributor payout: %s (%s)", c.GitLogin, alloc.Component)
	fe.Type = fee.Contributor
	fe.PayeeId = c.Id()
	fe.Currency = c.Currency
	fe.Amount = currency.Cents(alloc.AmountCents)
	fe.Status = fee.Pending
	if err := fe.Create(); err != nil {
		return fmt.Errorf("accrue fee for %s: %w", c.GitLogin, err)
	}
	log.Info("contributor-payout: accrued fee %s for %s (%s)", fe.Id(), c.GitLogin, c.Currency.ToString(fe.Amount))
	return nil
}

// calculatePeriodRevenue queries the transaction ledger for total revenue and
// per-component revenue attribution in the given period.
//
// Attribution is two-tier, both real (no flat fallback):
//  1. Exact — transactions tagged with a component (tx.Tags) attribute their
//     full amount to that component.
//  2. SBOM-weighted — revenue NOT tagged to a component is distributed across
//     the shipped components proportional to their SBOM weight (usage, then
//     lines). This is the deployed-images → SBOM-components → contributors
//     mapping. Untagged revenue is left unattributed only when there are no
//     SBOM entries to attribute it to.
func calculatePeriodRevenue(db *datastore.Datastore, start, end time.Time) (int64, map[string]int64, error) {
	var txns []*transaction.Transaction
	q := transaction.Query(db).
		Filter("Type=", "withdraw").
		Filter("CreatedAt>=", start).
		Filter("CreatedAt<", end)

	if _, err := q.GetAll(&txns); err != nil {
		return 0, nil, fmt.Errorf("query transactions: %w", err)
	}

	var totalRevenue, untagged int64
	componentRevenue := make(map[string]int64)

	for _, tx := range txns {
		amt := int64(tx.Amount)
		totalRevenue += amt
		if tx.Tags != "" {
			componentRevenue[tx.Tags] += amt // exact, transaction-tagged
		} else {
			untagged += amt
		}
	}

	// Distribute untagged revenue across components by real SBOM weight.
	if untagged > 0 {
		var entries []contribModel.SBOMEntry
		if _, err := contribModel.QuerySBOM(db).GetAll(&entries); err != nil {
			return 0, nil, fmt.Errorf("query sbom entries: %w", err)
		}
		weights := contribModel.ComponentWeightsFromSBOM(entries)
		for comp, rev := range contribModel.DistributeRevenue(untagged, weights) {
			componentRevenue[comp] += rev
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
func publishPayoutEvent(ctx context.Context, pub *events.Publisher, c *contribModel.Contributor, alloc contribModel.PayoutAllocation, txHash string, periodStart, periodEnd time.Time) {
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
			"tx_hash":        txHash,
			"period_start":   periodStart.Format("2006-01-02"),
			"period_end":     periodEnd.Format("2006-01-02"),
		},
	}

	if err := pub.Publish(ctx, events.SubjectContributorPayoutCalc, event); err != nil {
		log.Error("contributor-payout: failed to publish event for %s: %v", c.GitLogin, err)
	}
}
