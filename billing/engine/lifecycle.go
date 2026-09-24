package engine

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/money"
)

// subLockStripes bounds the memory of the per-subscription renewal lock to a
// fixed set of mutexes. Same subscription → same stripe → serialized; a rare hash
// collision only briefly serializes two unrelated renewals, which is harmless.
const subLockStripes = 256

var subLockMu [subLockStripes]sync.Mutex

// lockSubscription serializes the renewal step of one subscription WITHIN a
// process, returning the unlock func. It is taken once per step and never while
// another is held, so no step can wait on a stripe it holds. Commerce is
// single-writer per tenant (ReadWriteOnce PVC, Recreate), so this fully
// serializes real concurrent renewals (the cycle + a manual renew) — one invoice
// row, one charge attempt — closing the non-atomic findInvoiceForPeriod →
// issuePeriodInvoice window. The idempotencykey guards and the per-(subscription,
// period, attempt) processor idempotency key remain the money backstops (a
// cross-process racer still cannot double-charge).
func lockSubscription(subID string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(subID))
	mu := &subLockMu[h.Sum32()%subLockStripes]
	mu.Lock()
	return mu.Unlock
}

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
		sub.PeriodEnd = Advance(sub.TrialEnd, p)
	} else {
		sub.Status = subscription.Active
		sub.PeriodStart = now
		sub.PeriodEnd = Advance(now, p)
	}
}

// IsDue reports whether a subscription has reached the end of the period it has
// paid for: live (Live), and now at or past PeriodEnd. [PeriodStart, PeriodEnd]
// is the paid-through period, so a due subscription owes the NEXT one. It is the
// one definition of "due", shared by the cycle and Settle; how the next period is
// paid, or whether the subscription ends instead, is the Collection's.
//
// An externally collected subscription is never due. Its customer pays the
// processor directly, so an invoice built here would bill a period nobody owes us
// for. Its next period is recorded when that payment arrives, the way it was
// bought.
func IsDue(sub *subscription.Subscription, now time.Time) bool {
	if sub.Type == subscription.External {
		return false
	}
	return Live(sub) && !sub.PeriodEnd.IsZero() && !now.Before(sub.PeriodEnd)
}

// CreatePaidFirstInvoice builds the invoice for the period the subscription holds,
// marks it PAID by an already-settled charge (method + providerRef) and persists
// it. The caller has already taken the money for this period, so this only records
// the invoice as paid — it does NOT charge again. The row stays on the period it
// paid for; Settle bills the next one when this one ends. Idempotent per period:
// if an invoice for the held period already exists it is returned as-is (no
// duplicate, no state change), so a retried subscribe never double-invoices.
func CreatePaidFirstInvoice(db *datastore.Datastore, sub *subscription.Subscription, method, providerRef string) (*billinginvoice.BillingInvoice, error) {
	if existing, err := findInvoiceForPeriod(db, sub, sub.PeriodStart); err != nil {
		return nil, fmt.Errorf("failed to look up existing invoice for period: %w", err)
	} else if existing != nil {
		return existing, nil
	}

	// The first period has no period before it, so no usage rides on it.
	inv, err := draftPeriodInvoice(db, sub, sub.PeriodStart, sub.PeriodEnd, time.Time{}, time.Time{})
	if err != nil {
		return inv, err
	}
	if err := inv.MarkPaid(method, providerRef); err != nil {
		return inv, fmt.Errorf("failed to mark first invoice paid: %w", err)
	}
	if err := issuePeriodInvoice(db, inv); err != nil {
		return inv, err
	}
	sub.CurrentInvoiceId = inv.Id()
	return inv, nil
}

// draftPeriodInvoice builds an OPEN invoice for sub's plan over [start, end] in
// memory: the plan fee times billable seats, less the subscription's promo, plus
// the metered usage recorded between usageStart and usageEnd (none when that
// window is empty). Usage is billed in arrears, for the period that just ended,
// while the plan fee is billed for the period ahead. Nothing is stored and no
// number is assigned; issuePeriodInvoice does both.
//
// The fee is sub.Plan's: the plan as it was bought, at the price and interval
// the subscriber bought it at, whatever the catalog sells today.
func draftPeriodInvoice(db *datastore.Datastore, sub *subscription.Subscription, start, end, usageStart, usageEnd time.Time) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	inv.UserId = sub.UserId
	inv.SubscriptionId = sub.Id()
	inv.PeriodStart = start
	inv.PeriodEnd = end
	inv.Currency = sub.Plan.Currency

	// Plan fee × billable seats (1 for flat plans).
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
			PeriodStart: start,
			PeriodEnd:   end,
		})
	}

	// Metered usage (non-fatal: an aggregation error yields no usage). The window
	// is checked here because AggregateUsage reads a zero bound as unbounded.
	if !usageStart.IsZero() && usageEnd.After(usageStart) {
		if usageItems, _, err := AggregateUsage(db, sub.UserId, usageStart, usageEnd); err == nil {
			inv.LineItems = append(inv.LineItems, usageItems...)
		}
	}

	inv.RecalculateSubtotal()

	// The promo the subscription was bought under, priced off the PLAN fee only —
	// metered usage is real consumption and is never discounted by a plan promo.
	// Finalize() folds Discount into AmountDue, so this must land before it.
	if inv.Subtotal > 0 && sub.DiscountPercent > 0 {
		planFee := int64(0)
		for _, li := range inv.LineItems {
			if li.Type == billinginvoice.LineSubscription {
				planFee += li.Amount
			}
		}
		if d := DiscountCents(planFee, sub.DiscountPercent); d > 0 {
			inv.Discount = d
			inv.DiscountName = sub.DiscountName
		}
	}

	// Finalize (draft -> open)
	if err := inv.Finalize(); err != nil {
		return inv, fmt.Errorf("failed to finalize invoice: %w", err)
	}
	return inv, nil
}

// issuePeriodInvoice numbers and stores an invoice draftPeriodInvoice built.
func issuePeriodInvoice(db *datastore.Datastore, inv *billinginvoice.BillingInvoice) error {
	assignInvoiceNumber(db, inv)
	if err := inv.Create(); err != nil {
		return fmt.Errorf("failed to create invoice: %w", err)
	}
	return nil
}

// findInvoiceForPeriod returns the subscription's invoice for the period that
// starts at start, or nil if none exists. A period is one invoice however its end
// is later computed, so it is matched by its start alone, at second precision to
// be robust against sub-second serialization differences across the storage
// round-trip. A voided invoice bills nothing and is not the period's.
func findInvoiceForPeriod(db *datastore.Datastore, sub *subscription.Subscription, start time.Time) (*billinginvoice.BillingInvoice, error) {
	invs, err := invoicesOf(db, sub)
	if err != nil {
		return nil, err
	}
	for _, inv := range invs {
		if inv.PeriodStart.Unix() == start.Unix() && inv.Status != billinginvoice.Void {
			return inv, nil
		}
	}
	return nil, nil
}

// invoicesOf lists every invoice of the subscription. It reads under no
// ancestor: an invoice saved again after it was loaded by id is stored without
// the synckey parent it was created under, and missing it would bill its period
// twice. The namespace is the tenant boundary.
func invoicesOf(db *datastore.Datastore, sub *subscription.Subscription) ([]*billinginvoice.BillingInvoice, error) {
	invs := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).
		Filter("SubscriptionId=", sub.Id()).
		Filter("UserId=", sub.UserId)
	if _, err := q.GetAll(&invs); err != nil {
		return nil, err
	}
	return invs, nil
}

// loadInvoice reads the invoice with id into a row bound to db, so it can be
// written back.
func loadInvoice(db *datastore.Datastore, id string) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	if err := inv.GetById(id); err != nil {
		return nil, err
	}
	return inv, nil
}

// FirstInvoice is the subscription's earliest invoice by period start — the
// record of how it was bought — or nil when it has none. An invoice with no
// period (a one-off charge filed against the subscription) bills no period of
// it and is not considered.
func FirstInvoice(db *datastore.Datastore, sub *subscription.Subscription) (*billinginvoice.BillingInvoice, error) {
	invs, err := invoicesOf(db, sub)
	if err != nil {
		return nil, err
	}
	var first *billinginvoice.BillingInvoice
	for _, inv := range invs {
		if inv.PeriodStart.IsZero() || inv.PeriodEnd.IsZero() {
			continue
		}
		if first == nil || inv.PeriodStart.Before(first.PeriodStart) {
			first = inv
		}
	}
	return first, nil
}

// renewalInvoice picks, among invs, the invoice for the period starting at
// start that the renewal settles on, and the open ones it leaves behind. A paid
// one wins; then one a payment was attempted on — its period may end elsewhere
// when the plan's interval changed after it was issued, and it is followed to
// its own end rather than billed again; then an open one for exactly [start,
// end]; then a void or uncollectible one for exactly that period. Every other
// open invoice starting at start was never attempted and no longer matches the
// period being billed: stale, to be voided.
func renewalInvoice(invs []*billinginvoice.BillingInvoice, start, end time.Time) (pick *billinginvoice.BillingInvoice, stale []*billinginvoice.BillingInvoice) {
	var paid, tried, exact, closed *billinginvoice.BillingInvoice
	for _, inv := range invs {
		if inv.PeriodStart.Unix() != start.Unix() {
			continue
		}
		same := inv.PeriodEnd.Unix() == end.Unix()
		switch {
		case inv.Status == billinginvoice.Paid:
			if paid == nil || same {
				paid = inv
			}
		case inv.Status == billinginvoice.Open && attempted(inv):
			if tried == nil || same {
				tried = inv
			}
		case inv.Status == billinginvoice.Open && same && exact == nil:
			exact = inv
		case inv.Status != billinginvoice.Open && inv.Status != billinginvoice.Draft && same:
			closed = inv
		}
	}
	for _, c := range []*billinginvoice.BillingInvoice{paid, tried, exact, closed} {
		if c != nil {
			pick = c
			break
		}
	}
	for _, inv := range invs {
		if inv.PeriodStart.Unix() == start.Unix() && inv.Status == billinginvoice.Open && !attempted(inv) &&
			(pick == nil || inv.Id() != pick.Id()) {
			stale = append(stale, inv)
		}
	}
	return pick, stale
}

// assignInvoiceNumber sets a sequential per-org invoice number: one past the
// highest the org has issued, read from that one invoice rather than by loading
// them all. It reads under no ancestor, for the reason findInvoiceForPeriod does.
// If the read fails it falls back to 1.
func assignInvoiceNumber(db *datastore.Datastore, inv *billinginvoice.BillingInvoice) {
	top := make([]*billinginvoice.BillingInvoice, 0, 1)
	if _, err := billinginvoice.Query(db).Order("-Number").Limit(1).GetAll(&top); err == nil && len(top) == 1 {
		inv.SetNumber(top[0].Number + 1)
	} else {
		inv.SetNumber(1)
	}
}

// VoidOpen voids the open invoices a subscription still carries, for a row that
// has ended now: it is served no further period, so it owes none. What one
// collected is given back (ReturnPaid). Each is voided under its lock and read
// again inside it, so a payment landing at the same moment either finds it void
// or leaves it paid. An invoice whose lock another path holds is left, and so
// is one whose last payment attempt has no known outcome: the money may have
// moved. The billing cycle repeats that attempt under its key
// (ResolveCanceled), and it is logged for an operator.
func VoidOpen(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, p Prepaid, now time.Time) error {
	invs, err := invoicesOf(db, sub)
	if err != nil {
		return err
	}
	for _, listed := range invs {
		if listed.Status != billinginvoice.Open {
			continue
		}
		if listed.PendingKey != "" {
			log.Error("billing: ALERT subscription %s ended with the payment attempt %s on invoice %s of unknown outcome; "+
				"the billing cycle repeats it under its key", sub.Id(), listed.PendingKey, listed.Id())
			continue
		}
		if err := voidIdle(ctx, db, listed.Id(), p, now); err != nil {
			return err
		}
	}
	return nil
}

// voidIdle voids one invoice if, read under its lock, it is still open with no
// payment attempt in flight.
func voidIdle(ctx context.Context, db *datastore.Datastore, id string, p Prepaid, now time.Time) error {
	release, err := LockInvoice(db, id)
	if errors.Is(err, ErrInvoiceBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer release()
	inv, err := loadInvoice(db, id)
	if err != nil {
		return err
	}
	if inv.Status != billinginvoice.Open || inv.PendingKey != "" {
		return nil
	}
	if err := inv.MarkVoid(); err != nil {
		return err
	}
	inv.VoidedAt = now
	if err := ReturnPaid(ctx, db, inv, p, now); err != nil {
		return err
	}
	return inv.Update()
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
	oldCredit, err := proration(oldPlan.Price*currency.Cents(seats(&oldPlan, sub.Quantity)), fraction)
	if err != nil {
		return nil, err
	}
	// Charge for remaining portion of new plan (× its billable seats)
	newCharge, err := proration(newPlan.Price*currency.Cents(seats(newPlan, sub.Quantity)), fraction)
	if err != nil {
		return nil, err
	}

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
// proration returns the part of amount that fraction covers, rounded half away from zero.
//
// It is one function because a plan change computes two of these and subtracts them, and
// they have to round the same way. The spelling it replaces truncated both — the credit for
// the unused old plan, which is a cent the customer does not get back, and the charge for
// the new one, which is a cent we do not bill. Neither direction was chosen by anyone; they
// were just what int64() does to a fraction. Half away from zero is the rule the discount
// and coupon paths already use, so a plan change cannot disagree with a coupon about what a
// fraction of a cent is worth, and a downgrade credit is the upgrade charge reversed.
//
// A fraction that is not a number is REPORTED rather than converted: a period whose start
// and end are the same instant makes remaining/total a NaN, and NaN to int64 is undefined
// in Go. A proration line for an undefined number of cents is worse than no line at all.
func proration(amount currency.Cents, fraction float64) (int64, error) {
	rate, err := money.RateFromFloat(fraction)
	if err != nil {
		return 0, err
	}
	return money.ScaleMinor(int64(amount), rate)
}

func seats(p *plan.Plan, quantity int) int64 {
	if !p.PerSeat || quantity < 1 {
		return 1
	}
	return int64(quantity)
}

// Advance computes the next period end date based on the plan interval.
func Advance(from time.Time, p *plan.Plan) time.Time {
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
