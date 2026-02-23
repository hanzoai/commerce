package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/types"
)

// StartSubscription initializes a new subscription: sets the initial state,
// computes period dates, and handles trial logic.
func StartSubscription(sub *subscription.Subscription, p *plan.Plan) {
	now := time.Now()
	sub.Plan = *p
	sub.PlanId = p.Id()
	sub.Start = now

	if p.TrialPeriodDays > 0 {
		sub.Status = subscription.Trialing
		sub.TrialStart = now
		sub.TrialEnd = now.AddDate(0, 0, p.TrialPeriodDays)
		sub.PeriodStart = sub.TrialEnd
		sub.PeriodEnd = advancePeriod(sub.TrialEnd, p)
	} else {
		sub.Status = subscription.Active
		sub.PeriodStart = now
		sub.PeriodEnd = advancePeriod(now, p)
	}
}

// RenewSubscription generates an invoice for the current billing period
// and attempts to collect payment. Returns the invoice and collection result.
func RenewSubscription(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, burnCredits CreditBurner) (*billinginvoice.BillingInvoice, *CollectionResult, error) {
	// Generate invoice
	inv := billinginvoice.New(db)
	inv.UserId = sub.UserId
	inv.SubscriptionId = sub.Id()
	inv.PeriodStart = sub.PeriodStart
	inv.PeriodEnd = sub.PeriodEnd
	inv.Currency = sub.Plan.Currency

	// Add subscription line item (flat plan fee)
	if sub.Plan.Price > 0 {
		inv.LineItems = append(inv.LineItems, billinginvoice.LineItem{
			Id:          "li_plan_" + sub.PlanId,
			Type:        billinginvoice.LineSubscription,
			Description: sub.Plan.Name + " subscription",
			PlanId:      sub.PlanId,
			PlanName:    sub.Plan.Name,
			Amount:      int64(sub.Plan.Price),
			Currency:    sub.Plan.Currency,
			PeriodStart: sub.PeriodStart,
			PeriodEnd:   sub.PeriodEnd,
		})
	}

	// Add usage line items
	usageItems, _, err := AggregateUsage(db, sub.UserId, sub.PeriodStart, sub.PeriodEnd)
	if err != nil {
		// Non-fatal: invoice without usage
		_ = err
	} else {
		inv.LineItems = append(inv.LineItems, usageItems...)
	}

	// Calculate totals
	inv.RecalculateSubtotal()

	// Finalize (draft -> open)
	if err := inv.Finalize(); err != nil {
		return inv, nil, fmt.Errorf("failed to finalize invoice: %w", err)
	}

	// Persist invoice
	if err := inv.Create(); err != nil {
		return inv, nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	// Attempt collection
	result, err := CollectInvoice(ctx, db, inv, burnCredits)
	if err != nil {
		return inv, result, fmt.Errorf("collection error: %w", err)
	}

	// Update invoice after collection
	if err := inv.Update(); err != nil {
		return inv, result, fmt.Errorf("failed to update invoice: %w", err)
	}

	// Update subscription period
	if result.Success {
		sub.CurrentInvoiceId = inv.Id()
		sub.PeriodStart = sub.PeriodEnd
		sub.PeriodEnd = advancePeriod(sub.PeriodEnd, &sub.Plan)
	} else {
		sub.Status = subscription.PastDue
	}

	return inv, result, nil
}

// TransitionTrialToActive moves a trialing subscription to active.
func TransitionTrialToActive(sub *subscription.Subscription) error {
	if sub.Status != subscription.Trialing {
		return fmt.Errorf("subscription is not trialing, current status: %s", sub.Status)
	}
	sub.Status = subscription.Active
	return nil
}

// CancelSubscription cancels a subscription, either immediately or at period end.
func CancelSubscription(sub *subscription.Subscription, atPeriodEnd bool) error {
	if sub.Status == subscription.Canceled {
		return fmt.Errorf("subscription is already canceled")
	}

	now := time.Now()

	if atPeriodEnd {
		sub.EndCancel = true
		sub.CanceledAt = now
	} else {
		sub.Status = subscription.Canceled
		sub.Canceled = true
		sub.CanceledAt = now
		sub.Ended = now
	}

	return nil
}

// ReactivateSubscription reverses a pending cancellation.
func ReactivateSubscription(sub *subscription.Subscription) error {
	if sub.Status == subscription.Canceled && !sub.Ended.IsZero() {
		return fmt.Errorf("cannot reactivate a fully ended subscription")
	}

	sub.EndCancel = false
	sub.Canceled = false
	sub.CanceledAt = time.Time{}

	if sub.Status == subscription.Canceled {
		sub.Status = subscription.Active
	}

	return nil
}

// ChangePlan updates a subscription to a new plan. If prorate is true,
// a proration line item will be added to the current period's invoice.
func ChangePlan(sub *subscription.Subscription, newPlan *plan.Plan, prorate bool) (*billinginvoice.LineItem, error) {
	oldPlan := sub.Plan
	sub.Plan = *newPlan
	sub.PlanId = newPlan.Id()

	if !prorate {
		return nil, nil
	}

	// Calculate proration
	now := time.Now()
	totalDays := sub.PeriodEnd.Sub(sub.PeriodStart).Hours() / 24
	remainingDays := sub.PeriodEnd.Sub(now).Hours() / 24

	if totalDays <= 0 {
		return nil, nil
	}

	fraction := remainingDays / totalDays

	// Credit for unused portion of old plan
	oldCredit := int64(float64(oldPlan.Price) * fraction)
	// Charge for remaining portion of new plan
	newCharge := int64(float64(newPlan.Price) * fraction)

	net := newCharge - oldCredit

	item := &billinginvoice.LineItem{
		Id:          fmt.Sprintf("li_proration_%d", now.Unix()),
		Type:        billinginvoice.LineProration,
		Description: fmt.Sprintf("Proration: %s -> %s", oldPlan.Name, newPlan.Name),
		PlanId:      newPlan.Id(),
		PlanName:    newPlan.Name,
		Amount:      net,
		Currency:    newPlan.Currency,
		PeriodStart: now,
		PeriodEnd:   sub.PeriodEnd,
	}

	return item, nil
}

// advancePeriod computes the next period end date based on the plan interval.
func advancePeriod(from time.Time, p *plan.Plan) time.Time {
	count := p.IntervalCount
	if count <= 0 {
		count = 1
	}

	switch p.Interval {
	case types.Monthly:
		return from.AddDate(0, count, 0)
	case types.Yearly:
		return from.AddDate(count, 0, 0)
	default:
		// Default to monthly
		return from.AddDate(0, count, 0)
	}
}
