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
//
// It is idempotent per (subscription, period): a PastDue renewal re-runs the
// SAME period (the period only advances on a successful collection), so this
// must NEVER mint a second invoice for a period already invoiced. If an
// invoice for this exact period already exists it is returned as-is (no
// duplicate, no re-charge); retrying collection on an unpaid invoice is the
// dunning workflow's job (billing/workflows/dunning.go), not this generator's.
func RenewSubscription(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, burnCredits CreditBurner, chargeProvider ProviderCharger) (*billinginvoice.BillingInvoice, *CollectionResult, error) {
	// Idempotency guard: one invoice per (subscription, period). A PastDue
	// re-run returns the SAME open invoice WITHOUT re-charging (dunning, not this
	// generator, retries collection) — so a repeated renew never double-charges.
	existing, err := findInvoiceForPeriod(db, sub)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to look up existing invoice for period: %w", err)
	}
	if existing != nil {
		return existing, resultFromInvoice(existing), nil
	}

	// Generate a fresh, sequentially-numbered invoice for this period.
	inv, err := buildPeriodInvoice(db, sub)
	if err != nil {
		return inv, nil, err
	}

	// Attempt collection: credits -> balance -> the vaulted card (chargeProvider).
	result, err := CollectInvoice(ctx, db, inv, burnCredits, chargeProvider)
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

// CreatePaidFirstInvoice builds the subscription's FIRST-period invoice, marks it
// PAID by an already-settled external charge (method + providerRef), persists it,
// and advances the subscription to the next period. It is the upfront-collection
// analogue of RenewSubscription's success path: the caller (subscribe/card) has
// already charged the customer's card for this period synchronously, so this only
// records the invoice as paid — it does NOT charge again. Idempotent per period:
// if an invoice for the current period already exists it is returned as-is (no
// duplicate, no state change), so a retried subscribe never double-invoices.
func CreatePaidFirstInvoice(db *datastore.Datastore, sub *subscription.Subscription, method, providerRef string) (*billinginvoice.BillingInvoice, error) {
	if existing, err := findInvoiceForPeriod(db, sub); err != nil {
		return nil, fmt.Errorf("failed to look up existing invoice for period: %w", err)
	} else if existing != nil {
		return existing, nil
	}

	inv, err := buildPeriodInvoice(db, sub)
	if err != nil {
		return inv, err
	}
	if err := inv.MarkPaid(method, providerRef); err != nil {
		return inv, fmt.Errorf("failed to mark first invoice paid: %w", err)
	}
	if err := inv.Update(); err != nil {
		return inv, fmt.Errorf("failed to persist paid first invoice: %w", err)
	}

	// The first period is prepaid, so the next charge falls at the start of
	// period 2 — advance exactly as RenewSubscription does on a successful collect.
	sub.CurrentInvoiceId = inv.Id()
	sub.PeriodStart = sub.PeriodEnd
	sub.PeriodEnd = advancePeriod(sub.PeriodEnd, &sub.Plan)
	return inv, nil
}

// buildPeriodInvoice constructs, numbers, finalizes and persists a new invoice
// for the subscription's current period. The invoice number is a sequential
// per-org counter, mirroring the credit-note numbering in refunds.go.
func buildPeriodInvoice(db *datastore.Datastore, sub *subscription.Subscription) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	inv.UserId = sub.UserId
	inv.SubscriptionId = sub.Id()
	inv.PeriodStart = sub.PeriodStart
	inv.PeriodEnd = sub.PeriodEnd
	inv.Currency = sub.Plan.Currency

	// Add subscription line item: plan fee × billable seats (1 for flat plans).
	if sub.Plan.Price > 0 {
		n := seats(&sub.Plan, sub.Quantity)
		inv.LineItems = append(inv.LineItems, billinginvoice.LineItem{
			Id:          "li_plan_" + sub.PlanId,
			Type:        billinginvoice.LineSubscription,
			Description: sub.Plan.Name + " subscription",
			PlanId:      sub.PlanId,
			PlanName:    sub.Plan.Name,
			Quantity:    n,
			UnitPrice:   int64(sub.Plan.Price),
			Amount:      int64(sub.Plan.Price) * n,
			Currency:    sub.Plan.Currency,
			PeriodStart: sub.PeriodStart,
			PeriodEnd:   sub.PeriodEnd,
		})
	}

	// Add usage line items (non-fatal: an aggregation error yields no usage).
	if usageItems, _, err := AggregateUsage(db, sub.UserId, sub.PeriodStart, sub.PeriodEnd); err == nil {
		inv.LineItems = append(inv.LineItems, usageItems...)
	}

	// Calculate totals
	inv.RecalculateSubtotal()

	// Assign a sequential per-org invoice number BEFORE persisting.
	assignInvoiceNumber(db, inv)

	// Finalize (draft -> open)
	if err := inv.Finalize(); err != nil {
		return inv, fmt.Errorf("failed to finalize invoice: %w", err)
	}

	// Persist invoice
	if err := inv.Create(); err != nil {
		return inv, fmt.Errorf("failed to create invoice: %w", err)
	}

	return inv, nil
}

// findInvoiceForPeriod returns the existing invoice for this subscription's
// exact billing period, or nil if none exists. Periods are months/years apart,
// so PeriodStart/PeriodEnd are matched at second precision to be robust against
// sub-second serialization differences across the storage round-trip.
func findInvoiceForPeriod(db *datastore.Datastore, sub *subscription.Subscription) (*billinginvoice.BillingInvoice, error) {
	rootKey := db.NewKey("synckey", "", 1, nil)
	existing := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).Ancestor(rootKey).
		Filter("SubscriptionId=", sub.Id()).
		Filter("UserId=", sub.UserId)
	if _, err := q.GetAll(&existing); err != nil {
		return nil, err
	}
	for _, inv := range existing {
		if inv.PeriodStart.Unix() == sub.PeriodStart.Unix() &&
			inv.PeriodEnd.Unix() == sub.PeriodEnd.Unix() {
			return inv, nil
		}
	}
	return nil, nil
}

// assignInvoiceNumber sets a sequential per-org invoice number = (count of
// existing billing invoices) + 1. Mirrors the credit-note numbering pattern in
// refunds.go; if the count query fails it falls back to 1.
func assignInvoiceNumber(db *datastore.Datastore, inv *billinginvoice.BillingInvoice) {
	rootKey := db.NewKey("synckey", "", 1, nil)
	existing := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Ancestor(rootKey).GetAll(&existing); err == nil {
		inv.SetNumber(len(existing) + 1)
	} else {
		inv.SetNumber(1)
	}
}

// resultFromInvoice synthesizes a collection result from an invoice's persisted
// state — used when RenewSubscription returns an already-generated invoice so
// callers (e.g. the billing cycle) get a non-nil result reflecting whether the
// period is settled.
func resultFromInvoice(inv *billinginvoice.BillingInvoice) *CollectionResult {
	return &CollectionResult{
		Success:       inv.Status == billinginvoice.Paid,
		CreditUsed:    inv.CreditApplied,
		AmountCharged: inv.AmountPaid,
	}
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

	// Credit for unused portion of old plan (× its billable seats)
	oldCredit := int64(float64(oldPlan.Price) * float64(seats(&oldPlan, sub.Quantity)) * fraction)
	// Charge for remaining portion of new plan (× its billable seats)
	newCharge := int64(float64(newPlan.Price) * float64(seats(newPlan, sub.Quantity)) * fraction)

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

// seats returns the billable multiplier for a plan on a subscription: the
// subscription quantity (floored at 1) when the plan bills per seat, else 1.
func seats(p *plan.Plan, quantity int) int64 {
	if !p.PerSeat || quantity < 1 {
		return 1
	}
	return int64(quantity)
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
