package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/money"
)

// periodLockStripes bounds the memory of the per-(subscription, period) renewal
// lock to a fixed set of mutexes (no unbounded per-period growth). Same key → same
// stripe → serialized; a rare hash collision only briefly serializes two unrelated
// renewals, which is harmless.
const periodLockStripes = 256

var periodLockMu [periodLockStripes]sync.Mutex

// lockPeriod serializes concurrent collection of the SAME (subscription, period)
// WITHIN a process, returning the unlock func. Commerce is single-writer per tenant
// (ReadWriteOnce PVC, Recreate), so this fully serializes real concurrent renewals
// (the cron sweep + a manual renew) — one invoice row, one collection — closing the
// non-atomic findInvoiceForPeriod → buildPeriodInvoice window. The idempotencykey
// guard + the per-(sub, period, attempt) gateway idempotency key remain the
// money backstops (a cross-process racer still cannot double-charge).
func lockPeriod(sub *subscription.Subscription) func() {
	key := sub.Id() + "|" + strconv.FormatInt(sub.PeriodStart.Unix(), 10)
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	mu := &periodLockMu[h.Sum32()%periodLockStripes]
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

// RenewSubscription bills the period a due subscription owes next — in advance:
// the row holds the period it has paid for, and when that period ends the next one
// is invoiced and collected, and the row moves onto it only once it is paid. So a
// row is never served a period it has not paid for, and one canceled at the end of
// its period ends with the last period it paid for.
//
// It is idempotent per (subscription, period): a PastDue renewal re-runs the
// SAME period (the row only moves on a successful collection), so this must NEVER
// mint a second invoice for a period already invoiced. If an invoice for that
// period already exists it is never charged again here; retrying collection on an
// unpaid invoice is the dunning workflow's job (billing/workflows/dunning.go), not
// this generator's. What the existing invoice says still decides the row, because
// the row may not have been told: a paid one moves the row onto its period, and an
// unpaid one leaves it past due.
func RenewSubscription(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, prepaid Prepaid, chargeProvider ProviderCharger) (*billinginvoice.BillingInvoice, *CollectionResult, error) {
	// Serialize concurrent collection of this (subscription, period) in-process so
	// exactly ONE invoice row is built + collected for the period (books integrity).
	defer lockPeriod(sub)()
	now := time.Now()

	// A row canceled at the end of its period ends when that period does, is never
	// invoiced again, and owes nothing it will not be served.
	if ended(sub, now) {
		if err := VoidOpen(db, sub); err != nil {
			return nil, nil, fmt.Errorf("failed to void the ended subscription's open invoices: %w", err)
		}
		return nil, &CollectionResult{Error: "subscription was canceled at the end of its period"}, nil
	}

	// Idempotency (period): if the owed period already has an invoice, NEVER charge
	// it again here. Dunning retries collection via PayInvoice, not this generator.
	next := owed(sub, now)

	// When the period after the held one has already gone by, its invoice still
	// decides first: paid — however late — it moves the row on; unpaid, the period
	// was never served, so it is owed no longer and its invoice is void.
	if after := (period{sub.PeriodEnd, Advance(sub.PeriodEnd, &sub.Plan)}); after.start.Unix() != next.start.Unix() {
		prior, err := findInvoiceForPeriod(db, sub, after)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to look up the lapsed period's invoice: %w", err)
		}
		switch {
		case prior != nil && prior.Status == billinginvoice.Paid:
			settle(sub, prior)
			return prior, resultFromInvoice(prior), nil
		case prior != nil:
			if err := VoidOpen(db, sub); err != nil {
				return nil, nil, fmt.Errorf("failed to void the lapsed period's invoice: %w", err)
			}
		}
	}

	existing, err := findInvoiceForPeriod(db, sub, next)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to look up existing invoice for period: %w", err)
	}
	if existing != nil {
		settle(sub, existing)
		return existing, resultFromInvoice(existing), nil
	}

	// Only CREATE (and charge) a new period invoice when the held period is actually
	// over (its end has passed) or the subscription is already PastDue. This stops a
	// MANUAL renew from pre-billing periods early — N manual renews must never bill N
	// periods. The cycle already filters on IsDue; this is the authoritative gate for
	// every caller.
	if !IsDue(sub, now) {
		return nil, &CollectionResult{Error: "subscription period is not due for renewal"}, nil
	}

	// Atomic per-(subscription, period) guard: two concurrent renews collapse onto
	// ONE deterministic-id row (storage ON CONFLICT), so only the winner builds +
	// collects; the loser re-reads the now-existing invoice. The card charge (via
	// chargeProvider) ALSO carries a stable per-period Square idempotency key, so
	// even the narrow concurrent-first window can never double-charge.
	guardKey := "period:" + strconv.FormatInt(next.start.Unix(), 10)
	rec, replay, gerr := idempotencykey.Begin(db, "billing-renew:"+sub.Id(), guardKey)
	if gerr == nil && replay {
		if again, e := findInvoiceForPeriod(db, sub, next); e == nil && again != nil {
			settle(sub, again)
			return again, resultFromInvoice(again), nil
		}
		// Concurrent in-flight; its invoice is not yet visible. Do NOT run a second
		// collection alongside it.
		return nil, &CollectionResult{Error: "renewal already in progress for this period"}, nil
	}

	// Re-check after winning the guard: a racer in the concurrent-first window may
	// have persisted this period's invoice between our findInvoiceForPeriod above and
	// here. If so, use it — never build a second row for the same period.
	if again, e := findInvoiceForPeriod(db, sub, next); e == nil && again != nil {
		if rec != nil {
			_ = idempotencykey.Complete(rec, again.Id())
		}
		settle(sub, again)
		return again, resultFromInvoice(again), nil
	}

	// Generate a fresh, sequentially-numbered invoice: the owed period's fee in
	// advance, and the metered usage since the held period began, in arrears.
	inv, err := buildPeriodInvoice(db, sub, next, period{sub.PeriodStart, next.start})
	if err != nil {
		if rec != nil {
			_ = rec.Delete() // release the guard so a later attempt can rebuild
		}
		return inv, nil, err
	}

	// Attempt collection: prepaid (credits, then the balance), then the vaulted
	// card (chargeProvider).
	result, err := CollectInvoice(ctx, db, inv, prepaid, chargeProvider)
	if err != nil {
		return inv, result, fmt.Errorf("collection error: %w", err)
	}

	// Update invoice after collection
	if err := inv.Update(); err != nil {
		return inv, result, fmt.Errorf("failed to update invoice: %w", err)
	}

	settle(sub, inv)

	// The invoice now exists, so findInvoiceForPeriod short-circuits every future
	// renew BEFORE this guard — seal it (best-effort; it has done its job).
	if rec != nil {
		_ = idempotencykey.Complete(rec, inv.Id())
	}

	return inv, result, nil
}

// period is one billing period, [start, end).
type period struct{ start, end time.Time }

// owed is the period a due row pays for next: the one after the period it holds.
// A row whose next period has already passed — an active one whose renewal was
// missed, a past-due one that went unpaid through it — owes the period running
// now, and the periods that ended in between are not billed after the fact: a
// customer is never charged for a period it was not served, nor at once for every
// period since.
func owed(sub *subscription.Subscription, now time.Time) period {
	p := period{sub.PeriodEnd, Advance(sub.PeriodEnd, &sub.Plan)}
	if sub.PeriodEnd.IsZero() {
		return p
	}
	for !p.end.After(now) {
		p = period{p.end, Advance(p.end, &sub.Plan)}
	}
	return p
}

// IsDue reports whether a subscription's current period has elapsed and it is
// eligible to be (re)invoiced: Active or PastDue with a PeriodEnd in the past.
// The single definition of "due", shared by the billing-cycle filter and
// RenewSubscription's new-period gate.
//
// An externally collected subscription is never due. Its customer pays the
// processor directly, so an invoice built here would bill a period nobody owes us
// for, and collecting it would burn their credits, then their balance, then a card.
// Its next period is recorded when that payment arrives, not renewed.
func IsDue(sub *subscription.Subscription, now time.Time) bool {
	if sub.Type == subscription.External {
		return false
	}
	switch sub.Status {
	case subscription.Active, subscription.PastDue:
		return !sub.PeriodEnd.IsZero() && now.After(sub.PeriodEnd)
	default:
		return false
	}
}

// CreatePaidFirstInvoice builds the invoice for the period the subscription holds,
// marks it PAID by an already-settled charge (method + providerRef) and persists
// it. The caller has already taken the money for this period, so this only records
// the invoice as paid — it does NOT charge again. The row stays on the period it
// paid for; RenewSubscription bills the next one when this one ends. Idempotent per
// period: if an invoice for the held period already exists it is returned as-is (no
// duplicate, no state change), so a retried subscribe never double-invoices.
func CreatePaidFirstInvoice(db *datastore.Datastore, sub *subscription.Subscription, method, providerRef string) (*billinginvoice.BillingInvoice, error) {
	held := period{sub.PeriodStart, sub.PeriodEnd}
	if existing, err := findInvoiceForPeriod(db, sub, held); err != nil {
		return nil, fmt.Errorf("failed to look up existing invoice for period: %w", err)
	} else if existing != nil {
		return existing, nil
	}

	inv, err := buildPeriodInvoice(db, sub, held, period{})
	if err != nil {
		return inv, err
	}
	if err := inv.MarkPaid(method, providerRef); err != nil {
		return inv, fmt.Errorf("failed to mark first invoice paid: %w", err)
	}
	if err := inv.Update(); err != nil {
		return inv, fmt.Errorf("failed to persist paid first invoice: %w", err)
	}
	sub.CurrentInvoiceId = inv.Id()
	return inv, nil
}

// buildPeriodInvoice constructs, numbers, finalizes and persists a new invoice
// for the plan fee of period fee and the metered usage of period use (none when
// use is empty). The invoice number is a sequential per-org counter, mirroring the
// credit-note numbering in refunds.go.
func buildPeriodInvoice(db *datastore.Datastore, sub *subscription.Subscription, fee, use period) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	inv.UserId = sub.UserId
	inv.SubscriptionId = sub.Id()
	inv.PeriodStart = fee.start
	inv.PeriodEnd = fee.end
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
			PeriodStart: fee.start,
			PeriodEnd:   fee.end,
		})
	}

	// Add usage line items (non-fatal: an aggregation error yields no usage).
	if use.end.After(use.start) {
		if usageItems, _, err := AggregateUsage(db, sub.UserId, use.start, use.end); err == nil {
			inv.LineItems = append(inv.LineItems, usageItems...)
		}
	}

	// Calculate totals
	inv.RecalculateSubtotal()

	// The promo the subscription was bought under, priced off the PLAN fee only —
	// metered usage above is real consumption and is never discounted by a plan
	// promo. Finalize() folds Discount into AmountDue, so this must land before it.
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

// findInvoiceForPeriod returns the existing invoice for this subscription and
// the exact billing period p, or nil if none exists. Periods are months/years apart,
// so PeriodStart/PeriodEnd are matched at second precision to be robust against
// sub-second serialization differences across the storage round-trip.
//
// It reads by the subscription alone, under no ancestor: an invoice saved again
// after it was loaded by id is stored without the synckey parent it was created
// under, so an ancestor filter loses exactly the invoice a payment settled.
func findInvoiceForPeriod(db *datastore.Datastore, sub *subscription.Subscription, p period) (*billinginvoice.BillingInvoice, error) {
	existing := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).
		Filter("SubscriptionId=", sub.Id()).
		Filter("UserId=", sub.UserId)
	if _, err := q.GetAll(&existing); err != nil {
		return nil, err
	}
	// By the period's start alone, the renewal guard's own key: a period is one
	// invoice however its end is later computed.
	for _, inv := range existing {
		if inv.PeriodStart.Unix() == p.start.Unix() {
			return inv, nil
		}
	}
	return nil, nil
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

// ended ends a row canceled at the end of its period once that period is over:
// it is canceled as of the period's end and reports true. Any other row is
// untouched.
func ended(sub *subscription.Subscription, now time.Time) bool {
	if !sub.EndCancel || sub.PeriodEnd.IsZero() || now.Before(sub.PeriodEnd) {
		return false
	}
	if sub.Status != subscription.Active && sub.Status != subscription.PastDue {
		return false
	}
	sub.Status = subscription.Canceled
	sub.Canceled = true
	sub.Ended = sub.PeriodEnd
	return true
}

// VoidOpen voids the unpaid invoices a subscription still carries. A row that has
// ended is served no further period, so it owes none; an invoice something was
// already paid toward is left for a person to settle.
//
// Each is voided under the guard a payment of it takes, and read again inside it,
// so a payment landing at the same moment either finds it void or leaves it paid —
// never paid and then voided over.
func VoidOpen(db *datastore.Datastore, sub *subscription.Subscription) error {
	invs := make([]*billinginvoice.BillingInvoice, 0)
	keys, err := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id()).GetAll(&invs)
	if err != nil {
		return err
	}
	for i, listed := range invs {
		if listed.Status != billinginvoice.Open || listed.AmountPaid > 0 || i >= len(keys) {
			continue
		}
		if err := voidUnpaid(db, keys[i]); err != nil {
			return err
		}
	}
	return nil
}

// voidUnpaid voids one invoice if, read under its payment guard, it is still open
// with nothing paid toward it.
func voidUnpaid(db *datastore.Datastore, key datastore.Key) error {
	inv := billinginvoice.New(db)
	if err := inv.Get(key); err != nil {
		return err
	}
	rec, replay, err := idempotencykey.Begin(db, "billing-pay", "invoice:"+inv.Id())
	if err != nil {
		return err
	}
	if replay {
		return nil // a payment of it is in flight or done
	}
	defer func() { _ = rec.Delete() }()
	if err := inv.Get(key); err != nil {
		return err
	}
	if inv.Status != billinginvoice.Open || inv.AmountPaid > 0 {
		return nil
	}
	if err := inv.MarkVoid(); err != nil {
		return err
	}
	return inv.Update()
}

// settle moves the row by what its owed period's invoice says: paid moves it onto
// that period and makes it active, anything else leaves it past due. Only a row
// the cycle renews is moved; a canceled or trialing one is not the engine's.
func settle(sub *subscription.Subscription, inv *billinginvoice.BillingInvoice) {
	if sub.Status != subscription.Active && sub.Status != subscription.PastDue {
		return
	}
	if inv.Status != billinginvoice.Paid {
		// The row holds everything before the period it owes, so the period it owes
		// stays the one this invoice bills, however far a missed renewal moved it.
		sub.Status = subscription.PastDue
		sub.PeriodEnd = inv.PeriodStart
		return
	}
	// A row served nothing until it paid — past due, or paying only after the
	// period was over — is served the whole period it paid for from the moment it
	// paid, not what was left of the invoice's.
	sub.CurrentInvoiceId = inv.Id()
	sub.PeriodStart, sub.PeriodEnd = inv.PeriodStart, inv.PeriodEnd
	if !inv.PaidAt.IsZero() && (sub.Status == subscription.PastDue || !inv.PaidAt.Before(inv.PeriodEnd)) {
		sub.PeriodStart, sub.PeriodEnd = inv.PaidAt, Advance(inv.PaidAt, &sub.Plan)
	}
	if sub.Status == subscription.PastDue {
		sub.Status = subscription.Active
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
