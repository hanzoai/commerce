package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/types"
)

// RenewalGrace is how long past its paid-through date a subscription with no
// renewal attempt on record is still renewed by the cycle. A renewal charges
// exactly one period, starting at PeriodEnd. A row whose PeriodEnd is further
// back than this is overdue: the cycle was not running when it fell due, and
// Settle neither charges it nor changes it until the customer acts (Overdue).
// Lateness is the row's own: now against the end of the period it paid for.
const RenewalGrace = 72 * time.Hour

// Run is one pass of the billing cycle over an org, or one subscription's
// renewal a customer asked for.
type Run struct {
	// Now is the instant the pass settles against.
	Now time.Time
	// DryRun makes every decision and reports it, and writes nothing, moves no
	// money and emits nothing.
	DryRun bool
	// Asked is the customer renewing the subscription themselves: an overdue row
	// is renewed for the period from now. A declined renewal is still retried on
	// RetrySchedule; the customer pays it now by paying its invoice.
	Asked bool
	// Prepaid is the org's prepaid money: where what a voided or uncollectible
	// invoice collected is given back (ReturnPaid).
	Prepaid Prepaid
}

// RetrySchedule is when a declined renewal is charged again, measured from the
// first decline (the renewal invoice's DueDate): one, three and seven days after
// it. A cycle that reaches a retry late makes it once, and the next retry is the
// first point of the schedule still ahead, so a late cycle never fires several
// retries back to back. When the last one declines, the invoice is uncollectible
// and the subscription ends.
var RetrySchedule = []time.Duration{24 * time.Hour, 72 * time.Hour, 168 * time.Hour}

// Action is what one pass of the billing cycle did to one subscription.
type Action string

const (
	// Renewed: the next period is paid and the subscription moved onto it.
	Renewed Action = "renewed"
	// RenewalFailed: the first charge for the next period declined. The
	// subscription is past_due, keeps its tier, and is retried on RetrySchedule.
	RenewalFailed Action = "renewal_failed"
	// Retried: a retry paid the next period and the subscription moved onto it.
	Retried Action = "retried"
	// RetryFailed: a retry declined and another is scheduled.
	RetryFailed Action = "retry_failed"
	// CanceledAtPeriodEnd: the customer canceled at period end. Ended, nothing charged.
	CanceledAtPeriodEnd Action = "canceled_at_period_end"
	// ExpiredOverdue: the period a renewal invoice bills ended before it was
	// paid. The invoice is voided, never billed, and the subscription ends at its
	// paid period's end.
	ExpiredOverdue Action = "expired_overdue"
	// Uncollectible: the last retry declined. The invoice is uncollectible and the
	// subscription ended.
	Uncollectible Action = "uncollectible"
	// EndedWithParent: a bundle or seat row whose paying subscription has ended.
	EndedWithParent Action = "ended_with_parent"
	// Comped: never charged (an ecosystem org's own plan, provisioned on
	// enterprise terms). The period moved on to the one holding now and no
	// invoice was raised.
	Comped Action = "comped"
	// EndedNoCard: nothing on file can pay the next period: no card and no
	// recorded way it was bought. Ended at the end of its paid period, nothing
	// charged.
	EndedNoCard Action = "ended_no_card"
	// GiftEnded: a gift reached the end of the time it was granted for. Ended
	// then, nothing charged.
	GiftEnded Action = "gift_ended"
	// EndedOtherMode: bought in the org's other mode (test or live), so no card
	// on this org's processor pays it. Ended at its period end, nothing charged.
	EndedOtherMode Action = "ended_other_mode"
	// EndedInvoiceVoided: its renewal invoice was voided, so nothing will pay the
	// next period. Ended at the end of its paid period.
	EndedInvoiceVoided Action = "ended_invoice_voided"
	// ChargedAfterCancel: the customer canceled while the renewal's payment was
	// at the processor, and it landed. The subscription stays canceled; the
	// invoice keeps the payment, which is logged for a refund.
	ChargedAfterCancel Action = "charged_after_cancel"
	// Escalated: a payment attempt still has no known outcome when the retry
	// schedule is spent. The subscription is unpaid (it confers no tier), its
	// attempt stays recorded on the invoice, and an operator reconciles it with
	// the processor.
	Escalated Action = "escalated"
	// Overdue: its paid period ended more than RenewalGrace ago and no renewal
	// was attempted. Nothing is charged and nothing is written: it stays as it is
	// until the customer renews it (Run.Asked), cancels it or changes it, and the
	// time it was not renewed for is never billed.
	Overdue Action = "overdue"
	// Replaced: a lapsed subscription whose customer subscribed again. Ended at
	// its paid period's end, nothing charged for the time since.
	Replaced Action = "replaced"
	// Returned: money that landed for a period a canceled subscription is never
	// served — a prepaid payment made while it was outstanding, or a renewal paid
	// after it lapsed and before it was replaced — went back to the customer's
	// prepaid money.
	Returned Action = "returned"
	// Skipped: due, but nothing can be done on this pass. Step.Reason says why.
	Skipped Action = "skipped"
)

// Step is what Settle did to one subscription. An empty Action means nothing
// was due.
type Step struct {
	Action Action
	// Invoice is the renewal invoice the step charged, retried or ended on, when
	// there is one. In a dry run a new renewal invoice is an unsaved draft.
	Invoice *billinginvoice.BillingInvoice
	// AmountCharged is what was collected in this step, in cents. In a dry run it
	// is what would be collected if the payment went through.
	AmountCharged int64
	// Reason explains a decline, an end or a skip.
	Reason string
}

// Collection is how a subscription's renewals are paid. The caller decides it
// from how the subscription was bought, which the row and its first invoice
// record; the engine never guesses.
type Collection struct {
	// Choose names what pays a new attempt of amount on inv — PaidByCard or
	// PaidByPrepaid — without moving money. An error means nothing can pay it
	// now, which counts as a declined attempt, unless it wraps ErrChargeUnknown.
	// A dry run reports what Choose answers.
	Choose func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amount int64) (method string, err error)
	// Look finds out what became of a card attempt whose answer was lost,
	// without asking for money: the path for a subscription canceled since. Nil
	// means the attempt cannot be looked up here.
	Look Looker
	// Pay moves amount on method under an idempotency key: the attempt's
	// (inv.PendingKey) or the period's. It is asked again with the same method
	// and amount to repeat an attempt whose outcome was not reported, so it must
	// answer from the key when it has seen it. An error
	// wrapping ErrChargeUnknown means the money may have moved; the ref it returns
	// alongside is the processor's payment id, when it gave one. Choose and Pay
	// nil means nothing on file can pay the subscription: it ends at the end of
	// its paid period, and nothing is charged.
	Pay func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, method string, amount int64) (ref string, err error)
	// Comped subscriptions are never charged: once a period ends the row moves on
	// to the period holding now, and no invoice is raised.
	Comped bool
	// End, when set, ends the subscription at its period end with this action
	// instead of renewing it, and charges nothing: a gift that ran its granted
	// time, a row bought in the org's other mode.
	End Action
	// Reason says why, for the report.
	Reason string
}

// Looked is what became of a payment attempt, found without asking for money.
type Looked struct {
	// Known is whether the processor could say.
	Known bool
	// Landed is whether the money moved.
	Landed bool
	// Ref is the processor's id for the payment, when it landed and gave one.
	Ref string
}

// Looker finds out what became of inv's recorded payment attempt (PendingKey,
// PendingRef) without asking for money again.
type Looker func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice) (Looked, error)

// Settle takes the one step the billing cycle takes for a subscription at
// run.Now.
//
// [PeriodStart, PeriodEnd] is the period the subscription has paid for, and a
// paid invoice covers it. Once now reaches PeriodEnd (IsDue), the subscription
// owes the next period, [PeriodEnd, PeriodEnd + interval] of the plan it bought
// (sub.Plan: its interval, its price), and Settle does exactly one of:
//
//   - the next period's invoice is already paid (the customer paid it, or a
//     payment landed and the subscription write did not): move onto the period.
//   - its invoice is uncollectible or void: end the subscription.
//
// Every step that pays, voids or advances on an existing renewal invoice holds
// its lock (LockInvoice) and reads it again first; an invoice another path holds
// is left for the next run.
//
//   - a payment attempt on its invoice has no known outcome: repeat it under its
//     key before anything else, and escalate it when nothing can repeat it.
//   - the customer canceled at period end: end at PeriodEnd, void any open
//     renewal invoice, charge nothing.
//   - the Collection ends it (a gift that ran its time, a row of the org's other
//     mode) or nothing can pay it (c.Pay is nil): end at PeriodEnd, charge
//     nothing.
//   - it is comped: move on to the period holding now, charge nothing.
//   - a payment was attempted on its invoice: retry it on RetrySchedule.
//   - it is overdue — PeriodEnd is more than RenewalGrace ago and no payment
//     was attempted: nothing is charged or written (Overdue), unless the
//     customer asked (run.Asked), when the row is carried to now without a
//     charge and the period from now is billed. Missed periods are never billed.
//   - otherwise: invoice the next period and collect it through c.Pay, once.
//
// The subscription moves onto the next period only when its invoice is paid.
// The invoice carries the plan fee for the next period and the metered usage of
// the period that just ended. Settle persists what it changes. A dry run makes
// the same decisions against the rows in memory, writes nothing, moves no money,
// and reports each payment as c.Choose answers it.
func Settle(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, run Run, c Collection) (*Step, error) {
	if !IsDue(sub, run.Now) {
		return &Step{}, nil
	}
	invs, err := invoicesOf(db, sub)
	if err != nil {
		return nil, fmt.Errorf("look up invoices: %w", err)
	}
	return settle(ctx, db, sub, invs, run, c)
}

// Live reports whether the billing cycle settles a subscription: active, past
// due, or unpaid (escalated: a payment attempt with no known outcome, which the
// cycle keeps repeating under its key until the processor answers or an
// operator voids the invoice).
func Live(sub *subscription.Subscription) bool {
	switch sub.Status {
	case subscription.Active, subscription.PastDue, subscription.Unpaid:
		return true
	}
	return false
}

// Lapsed reports whether sub no longer confers its plan at now though no step
// has ended it: an active row whose paid period ended more than RenewalGrace
// ago (overdue), or a past_due or unpaid one past the grace window and the whole
// retry schedule after it. Nothing is charged for the time since; its customer
// renews it, or subscribes again and the new subscription replaces it.
func Lapsed(sub *subscription.Subscription, now time.Time) bool {
	if sub.PeriodEnd.IsZero() {
		return false
	}
	switch sub.Status {
	case subscription.Active:
		return now.Sub(sub.PeriodEnd) > RenewalGrace
	case subscription.PastDue, subscription.Unpaid:
		return now.Sub(sub.PeriodEnd) > RenewalGrace+RetrySchedule[len(RetrySchedule)-1]
	}
	return false
}

// ResolveCanceled settles a payment attempt with no known outcome that a
// canceled subscription's invoice still carries, without ever asking for money
// again: the customer canceled, so no new payment may be made for them.
//
//   - A card attempt is looked up (c.Look): a payment that landed is recorded
//     on its invoice and reported for a refund (ChargedAfterCancel); one that
//     never did clears the attempt and voids the invoice.
//   - A prepaid attempt is repeated under the period's ref: the ledger answers
//     with the posting it holds, or makes it, and whatever it took is given
//     straight back (ReturnPaid) as the invoice is voided (Returned).
//   - Anything that cannot be settled so stays recorded on the open invoice and
//     is reported, with an alert, on every run until an operator reconciles it
//     (POST /v1/billing/invoices/:id/void-unresolved).
//
// A subscription replaced after it lapsed (Replaced) is never served past its
// paid period, so money one of its invoices collected toward a later period —
// a renewal paid after the lapse, before the cycle moved the row on — is given
// back (ReturnPaid) and an open invoice voided (Returned).
//
// Any other subscription, or one with nothing to settle, is left alone. A dry
// run reports what it would settle and moves nothing.
func ResolveCanceled(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, run Run, c Collection) (*Step, error) {
	if sub.Status != subscription.Canceled {
		return &Step{}, nil
	}
	invs, err := invoicesOf(db, sub)
	if err != nil {
		return nil, fmt.Errorf("look up invoices: %w", err)
	}
	for _, listed := range invs {
		if listed.Status != billinginvoice.Open || listed.PendingKey == "" {
			continue
		}
		if run.DryRun {
			return &Step{Action: Skipped, Invoice: listed,
				Reason: "canceled with a payment attempt of unknown outcome; the live run looks it up and asks for no money"}, nil
		}
		release, err := LockInvoice(db, listed.Id())
		if errors.Is(err, ErrInvoiceBusy) {
			return &Step{Action: Skipped, Invoice: listed, Reason: "a payment or change on its invoice is in progress"}, nil
		}
		if err != nil {
			return nil, err
		}
		defer release()
		inv, err := loadInvoice(db, listed.Id())
		if err != nil {
			return nil, fmt.Errorf("read invoice %s under its lock: %w", listed.Id(), err)
		}
		if inv.Status != billinginvoice.Open || inv.PendingKey == "" {
			return &Step{}, nil
		}
		s := &settlement{ctx: ctx, db: db, sub: sub, now: run.Now, c: c, run: run}
		switch {
		case inv.PendingMethod == PaidByPrepaid && c.Pay != nil:
			return s.returnPrepaid(inv)
		case inv.PendingMethod != PaidByPrepaid && c.Look != nil:
			return s.lookCanceled(inv)
		}
		return s.unresolved(inv, "nothing here can look the attempt up")
	}
	if reason, _ := sub.Metadata["endReason"].(string); reason == string(Replaced) {
		for _, listed := range invs {
			if unserved(sub, listed) {
				return returnUnserved(ctx, db, sub, listed.Id(), run)
			}
		}
	}
	return &Step{}, nil
}

// unserved reports whether inv collected money toward a period sub, ended, is
// never served, has no attempt in flight and has not given the money back.
func unserved(sub *subscription.Subscription, inv *billinginvoice.BillingInvoice) bool {
	if _, done := inv.Metadata["returnedCents"]; done {
		return false
	}
	return (inv.Status == billinginvoice.Paid || inv.Status == billinginvoice.Open) &&
		inv.AmountPaid > 0 && inv.PendingKey == "" && !inv.PeriodStart.Before(sub.PeriodEnd)
}

// returnUnserved gives back what invoice id collected toward a period sub is
// never served, under the invoice's lock, voiding it when it is open.
func returnUnserved(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, id string, run Run) (*Step, error) {
	const reason = "replaced after a payment toward a later period; what it collected went back to the customer's prepaid money, for a refund review"
	if run.DryRun {
		inv, err := loadInvoice(db, id)
		if err != nil {
			return nil, err
		}
		return &Step{Action: Returned, Invoice: inv, Reason: reason}, nil
	}
	release, err := LockInvoice(db, id)
	if errors.Is(err, ErrInvoiceBusy) {
		return &Step{Action: Skipped, Reason: "a payment or change on invoice " + id + " is in progress"}, nil
	}
	if err != nil {
		return nil, err
	}
	defer release()
	inv, err := loadInvoice(db, id)
	if err != nil {
		return nil, fmt.Errorf("read invoice %s under its lock: %w", id, err)
	}
	if !unserved(sub, inv) {
		return &Step{}, nil
	}
	if inv.Status == billinginvoice.Open {
		if err := inv.MarkVoid(); err != nil {
			return nil, err
		}
		inv.VoidedAt = run.Now
	}
	if err := ReturnPaid(ctx, db, inv, run.Prepaid, run.Now); err != nil {
		return nil, err
	}
	if err := inv.Update(); err != nil {
		return nil, fmt.Errorf("record the return on invoice %s: %w", inv.Id(), err)
	}
	return &Step{Action: Returned, Invoice: inv, Reason: reason}, nil
}

// lookCanceled settles a canceled subscription's card attempt by looking it up.
func (s *settlement) lookCanceled(inv *billinginvoice.BillingInvoice) (*Step, error) {
	looked, err := s.c.Look(s.ctx, s.db, inv)
	switch {
	case err != nil:
		return s.unresolved(inv, err.Error())
	case !looked.Known:
		return s.unresolved(inv, "the processor could not say what became of it")
	case looked.Landed:
		method, amount := inv.PendingMethod, inv.PendingAmount
		clearPending(inv)
		inv.AttemptCount++
		inv.AmountPaid += amount
		if err := markPaid(inv, method, looked.Ref, s.now, &CollectionResult{}); err != nil {
			return nil, err
		}
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("record the payment found on invoice %s: %w", inv.Id(), err)
		}
		return s.canceledMeanwhile(inv, amount)
	}
	clearPending(inv)
	inv.AttemptCount++
	if err := s.voidInvoice(inv); err != nil {
		return nil, err
	}
	return &Step{Action: Skipped, Invoice: inv, Reason: "canceled; the attempt took no money and its invoice is voided"}, nil
}

// returnPrepaid settles a canceled subscription's prepaid attempt: it is
// repeated under the period's ref, and whatever the ledger took goes back.
func (s *settlement) returnPrepaid(inv *billinginvoice.BillingInvoice) (*Step, error) {
	amount := inv.PendingAmount
	_, err := s.c.Pay(s.ctx, s.db, inv, inv.PendingMethod, amount)
	switch {
	case err != nil && errors.Is(err, ErrChargeUnknown):
		return s.unresolved(inv, err.Error())
	case err != nil:
		clearPending(inv)
		inv.AttemptCount++
		if err := s.voidInvoice(inv); err != nil {
			return nil, err
		}
		return &Step{Action: Skipped, Invoice: inv, Reason: "canceled; the attempt took no money and its invoice is voided"}, nil
	}
	clearPending(inv)
	inv.AttemptCount++
	inv.AmountPaid += amount
	if err := s.voidInvoice(inv); err != nil {
		return nil, err
	}
	return &Step{Action: Returned, Invoice: inv, Reason: "canceled; what the attempt took went back to the customer's prepaid money"}, nil
}

// unresolved keeps a canceled subscription's attempt recorded on its open
// invoice and reports it for an operator.
func (s *settlement) unresolved(inv *billinginvoice.BillingInvoice, why string) (*Step, error) {
	log.Error("billing cycle: ALERT subscription %s is canceled and the payment attempt %s on invoice %s has no known outcome (%s); "+
		"reconcile it with the processor and void it (POST /v1/billing/invoices/%s/void-unresolved)", s.sub.Id(), inv.PendingKey, inv.Id(), why, inv.Id())
	return &Step{Action: Skipped, Invoice: inv,
		Reason: "canceled with a payment attempt of unknown outcome (" + why + "); an operator reconciles it"}, nil
}

// settle decides and takes the step for a due subscription.
func settle(ctx context.Context, db *datastore.Datastore, sub *subscription.Subscription, invs []*billinginvoice.BillingInvoice, run Run, c Collection) (*Step, error) {
	now := run.Now
	s := &settlement{
		ctx: ctx, db: db, sub: sub, now: now, c: c, run: run,
		start: sub.PeriodEnd,
		end:   Advance(sub.PeriodEnd, &sub.Plan),
	}
	defer lockSubscription(sub.Id())()

	pick, stale := renewalInvoice(invs, s.start, s.end)
	if !run.DryRun {
		for _, old := range stale {
			if err := s.voidStale(old); err != nil {
				return nil, err
			}
		}
	}
	var inv *billinginvoice.BillingInvoice
	if pick != nil {
		var err error
		if inv, err = loadInvoice(db, pick.Id()); err != nil {
			return nil, fmt.Errorf("look up renewal invoice: %w", err)
		}
		// An invoice issued before the plan's interval changed pays the period it
		// was issued for; the subscription follows it there.
		s.end = inv.PeriodEnd
	}
	if inv != nil && !run.DryRun {
		release, err := LockInvoice(db, inv.Id())
		if errors.Is(err, ErrInvoiceBusy) {
			return &Step{Action: Skipped, Invoice: inv, Reason: "a payment or change on its renewal invoice is in progress"}, nil
		}
		if err != nil {
			return nil, err
		}
		defer release()
		if inv, err = loadInvoice(db, inv.Id()); err != nil {
			return nil, fmt.Errorf("read renewal invoice under its lock: %w", err)
		}
	}
	late := now.Sub(sub.PeriodEnd) > RenewalGrace
	switch {
	case inv != nil && inv.Status == billinginvoice.Paid:
		return s.advance(inv, Renewed, 0)
	case inv != nil && inv.Status == billinginvoice.Uncollectible:
		return s.close(Uncollectible, inv, now, "the renewal invoice is uncollectible")
	case inv != nil && inv.Status == billinginvoice.Void:
		return s.close(EndedInvoiceVoided, inv, sub.PeriodEnd, "its renewal invoice "+inv.Id()+" was voided")
	case inv != nil && inv.Status != billinginvoice.Open:
		return &Step{Action: Skipped, Invoice: inv,
			Reason: fmt.Sprintf("renewal invoice %s is %s", inv.Id(), inv.Status)}, nil
	case inv != nil && inv.PendingKey != "" && c.Pay != nil:
		// A payment attempt with no known outcome is resolved before anything
		// else: repeating it under its key moves no new money, and an invoice
		// whose charge may have landed is never voided by a cancel or an end.
		return s.retry(inv)
	case inv != nil && inv.PendingKey != "":
		return s.escalate(inv, "the attempt cannot be repeated: "+c.Reason)
	case sub.EndCancel:
		return s.void(inv, CanceledAtPeriodEnd, "canceled at period end")
	case c.End != "":
		return s.void(inv, c.End, c.Reason)
	case c.Comped:
		return s.comp(inv)
	case c.Pay == nil:
		return s.void(inv, EndedNoCard, c.Reason)
	case late && run.Asked && (inv == nil || !attempted(inv) || !now.Before(inv.PeriodEnd)):
		// The customer renews a row that is overdue — its missed period, even one
		// whose renewal once declined, is over and never billed: renew from now.
		return s.carry(inv)
	case inv != nil && attempted(inv):
		// A charge was attempted: from here the retry schedule decides, never the
		// grace window, so an invoice whose charge may have landed is never voided.
		return s.retry(inv)
	case late:
		return &Step{Action: Overdue, Invoice: inv, Reason: fmt.Sprintf(
			"paid through %s, more than %s ago, and never renewed; nothing is charged until the customer renews it",
			sub.PeriodEnd.UTC().Format(time.RFC3339), RenewalGrace)}, nil
	case inv != nil:
		// Issued and never charged: the process stopped between the two.
		return s.attempt(inv, Renewed, RenewalFailed)
	default:
		return s.renew()
	}
}

// End closes a subscription: canceled, ended at `at`, and the reason recorded on
// the row. CanceledAt keeps the moment a customer asked to cancel, when they did.
func End(sub *subscription.Subscription, at, now time.Time, reason Action) {
	sub.Status = subscription.Canceled
	sub.Canceled = true
	if sub.CanceledAt.IsZero() {
		sub.CanceledAt = now
	}
	sub.Ended = at
	if sub.Metadata == nil {
		sub.Metadata = types.Map{}
	}
	sub.Metadata["endReason"] = string(reason)
}

// settlement is one Settle call: the subscription and the period it owes.
type settlement struct {
	ctx context.Context
	db  *datastore.Datastore
	sub *subscription.Subscription
	now time.Time
	c   Collection
	run Run
	// start and end bound the period being billed: the one after PeriodEnd.
	start, end time.Time
}

// carry renews an overdue subscription its customer asked to renew. The time it
// was not renewed for is never billed: the row is carried to now, charging
// nothing for it, and the period from now is invoiced and collected. An invoice
// left for the missed period is voided.
//
// A missed period end is carried once. Its guard (keyed by the period end it
// carries from) stops a second process, and the row is carried only when it
// still ends where this step read it — a renewal that moved it meanwhile wins,
// and this one charges nothing.
func (s *settlement) carry(inv *billinginvoice.BillingInvoice) (*Step, error) {
	from := s.sub.PeriodEnd
	if !s.run.DryRun {
		rec, replay, err := idempotencykey.Begin(s.db, "billing-carry:"+s.sub.Id(), "from:"+strconv.FormatInt(from.Unix(), 10))
		if err != nil {
			return nil, fmt.Errorf("take the carry guard of subscription %s: %w", s.sub.Id(), err)
		}
		if replay {
			return &Step{Action: Skipped, Reason: "a renewal of this subscription from " + from.UTC().Format(time.RFC3339) + " is under way or done"}, nil
		}
		step, err := s.carryFrom(inv, from)
		if err != nil {
			_ = rec.Delete()
			return nil, err
		}
		_ = idempotencykey.Complete(rec, string(step.Action))
		return step, nil
	}
	return s.carryFrom(inv, from)
}

// carryFrom moves the row from the period end it was read at to now, then
// renews it. See carry.
func (s *settlement) carryFrom(inv *billinginvoice.BillingInvoice, from time.Time) (*Step, error) {
	now := s.now
	unmoved := func(row *subscription.Subscription) bool { return row.PeriodEnd.Unix() == from.Unix() }
	switch err := s.saveWhen(unmoved, func(row *subscription.Subscription) { row.PeriodStart, row.PeriodEnd = now, now }); {
	case errors.Is(err, errCanceled):
		return s.canceledMeanwhile(nil, 0)
	case errors.Is(err, errMoved):
		return &Step{Action: Skipped, Reason: "renewed meanwhile; nothing more is charged"}, nil
	case err != nil:
		return nil, fmt.Errorf("carry subscription %s to now: %w", s.sub.Id(), err)
	}
	if err := s.voidInvoice(inv); err != nil {
		return nil, err
	}
	s.start, s.end = now, Advance(now, &s.sub.Plan)
	return s.renew()
}

// renew issues the next period's invoice and charges it.
func (s *settlement) renew() (*Step, error) {
	inv, err := draftPeriodInvoice(s.db, s.sub, s.start, s.end, s.sub.PeriodStart, s.sub.PeriodEnd)
	if err != nil {
		return nil, err
	}
	inv.DueDate = s.now
	if s.run.DryRun {
		return s.attempt(inv, Renewed, RenewalFailed)
	}

	// Two processes renewing the same period collapse onto one guard row
	// (deterministic id, storage upsert), so only one issues the invoice.
	rec, replay, gerr := idempotencykey.Begin(s.db, "billing-renew:"+s.sub.Id(),
		"period:"+strconv.FormatInt(s.start.Unix(), 10))
	if gerr == nil && replay {
		return &Step{Action: Skipped, Reason: "another process is renewing this period"}, nil
	}
	// Re-check after winning the guard: a racer may have issued the invoice between
	// the lookup in Settle and here. The next cycle settles it.
	if again, err := findInvoiceForPeriod(s.db, s.sub, s.start); err == nil && again != nil {
		if rec != nil {
			_ = idempotencykey.Complete(rec, again.Id())
		}
		return &Step{Action: Skipped, Invoice: again, Reason: "another process issued this period's invoice"}, nil
	}
	if err := issuePeriodInvoice(s.db, inv); err != nil {
		if rec != nil {
			_ = rec.Delete() // release the guard so a later cycle can issue it
		}
		return nil, err
	}
	if rec != nil {
		_ = idempotencykey.Complete(rec, inv.Id())
	}
	release, err := LockInvoice(s.db, inv.Id())
	if errors.Is(err, ErrInvoiceBusy) {
		return &Step{Action: Skipped, Invoice: inv, Reason: "a payment or change on its renewal invoice is in progress"}, nil
	}
	if err != nil {
		return nil, err
	}
	defer release()
	return s.attempt(inv, Renewed, RenewalFailed)
}

// retry charges a renewal invoice again after an earlier attempt: at once when
// that attempt's outcome is unknown, else when its next point on RetrySchedule
// has come. It reports the wait otherwise.
func (s *settlement) retry(inv *billinginvoice.BillingInvoice) (*Step, error) {
	if inv.PendingKey != "" {
		// An attempt whose outcome is not known is resolved first, whatever the
		// schedule says: repeating it under its key moves no new money.
		return s.attempt(inv, Retried, RetryFailed)
	}
	scheduled := false
	if inv.DueDate.IsZero() {
		// Declined before retries were scheduled: the schedule runs from the
		// last attempt, the only decline on record.
		inv.DueDate = inv.LastAttemptAt
		if inv.DueDate.IsZero() {
			inv.DueDate = s.now
		}
		scheduled = true
	}
	if inv.NextAttemptAt.IsZero() {
		inv.NextAttemptAt = nextRetry(inv.DueDate, inv.DueDate)
		scheduled = true
	}
	if !s.now.Before(inv.NextAttemptAt) {
		return s.attempt(inv, Retried, RetryFailed)
	}

	// Waiting for the next retry. Record a schedule derived just now, and the
	// past_due status a lost write may have left active.
	if !s.run.DryRun && scheduled {
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("schedule retries on invoice %s: %w", inv.Id(), err)
		}
	}
	if s.sub.Status != subscription.PastDue {
		if err := s.save(pastDue); errors.Is(err, errCanceled) {
			return s.canceledMeanwhile(inv, 0)
		} else if err != nil {
			return nil, fmt.Errorf("mark subscription %s past due: %w", s.sub.Id(), err)
		}
	}
	return &Step{Action: Skipped, Invoice: inv,
		Reason: "next retry at " + inv.NextAttemptAt.UTC().Format(time.RFC3339)}, nil
}

// attempt makes one payment attempt on inv through the subscription's
// Collection (Pay). On success the subscription moves onto the paid period; on
// a decline the next retry is scheduled, or, when the schedule is spent, the
// invoice is uncollectible and the subscription ends; an outcome nobody
// reported is repeated, under the same key, on the next run.
func (s *settlement) attempt(inv *billinginvoice.BillingInvoice, paid, declined Action) (*Step, error) {
	if inv.DueDate.IsZero() {
		inv.DueDate = s.now
	}
	if inv.PendingKey == "" && !s.now.Before(inv.PeriodEnd) {
		// The period this invoice would pay for is over: it is never billed. An
		// attempt already at the processor is still resolved, above all else.
		return s.void(inv, ExpiredOverdue, "the period it would pay for ended "+inv.PeriodEnd.UTC().Format(time.RFC3339))
	}
	if s.run.DryRun {
		amount := inv.AmountDue - inv.AmountPaid
		if inv.PendingKey != "" {
			amount = inv.PendingAmount
		} else if amount > 0 {
			if s.c.Choose == nil {
				return &Step{Action: declined, Invoice: inv, Reason: "nothing on file pays this invoice"}, nil
			}
			if _, err := s.c.Choose(s.ctx, s.db, inv, amount); err != nil {
				return &Step{Action: declined, Invoice: inv, Reason: err.Error()}, nil
			}
		}
		return s.advance(inv, paid, amount)
	}

	if inv.PendingKey == "" {
		// The customer may have canceled since this run read the row: a new charge
		// is asked for only against the row as it stands now.
		row := subscription.New(s.db)
		if err := row.GetById(s.sub.Id()); err == nil && (row.EndCancel || row.Status == subscription.Canceled) {
			s.sub.EndCancel, s.sub.Canceled, s.sub.CanceledAt = row.EndCancel, row.Canceled, row.CanceledAt
			if row.Status == subscription.Canceled {
				if err := s.voidInvoice(inv); err != nil {
					return nil, err
				}
				return &Step{Action: Skipped, Invoice: inv, Reason: "the subscription was canceled while its renewal was being prepared"}, nil
			}
			return s.void(inv, CanceledAtPeriodEnd, "canceled at period end")
		}
	}
	res, err := Pay(s.ctx, s.db, inv, s.c, s.now)
	if err != nil {
		return nil, err
	}
	if res.Unknown {
		if nextRetry(inv.DueDate, s.now).IsZero() {
			return s.escalate(inv, res.Error)
		}
		// Neither charged nor declined as far as anyone knows. The next run repeats
		// this attempt at the same count, so under the same idempotency key: the
		// processor answers with this charge if it landed and makes it if not.
		inv.NextAttemptAt = s.now
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("record unknown charge on invoice %s: %w", inv.Id(), err)
		}
		if err := s.save(pastDue); errors.Is(err, errCanceled) {
			return s.canceledMeanwhile(inv, 0)
		} else if err != nil {
			return nil, fmt.Errorf("mark subscription %s past due: %w", s.sub.Id(), err)
		}
		return &Step{Action: Skipped, Invoice: inv,
			Reason: res.Error + "; the next run repeats this charge under the same idempotency key"}, nil
	}
	if res.Success {
		if err := inv.Update(); err != nil {
			// The payment landed. The next run finds the attempt still recorded as
			// pending, repeats it under its key, gets this payment back and records
			// it; nothing is charged twice.
			return nil, fmt.Errorf("record paid renewal invoice %s: %w", inv.Id(), err)
		}
		return s.advance(inv, paid, res.AmountCharged)
	}

	next := nextRetry(inv.DueDate, s.now)
	if next.IsZero() {
		inv.NextAttemptAt = time.Time{}
		if err := inv.MarkUncollectible(); err != nil {
			return nil, err
		}
		if err := ReturnPaid(s.ctx, s.db, inv, s.run.Prepaid, s.now); err != nil {
			return nil, err
		}
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("mark invoice %s uncollectible: %w", inv.Id(), err)
		}
		return s.close(Uncollectible, inv, s.now, res.Error)
	}
	inv.NextAttemptAt = next
	if err := inv.Update(); err != nil {
		return nil, fmt.Errorf("record declined attempt on invoice %s: %w", inv.Id(), err)
	}
	if err := s.save(pastDue); errors.Is(err, errCanceled) {
		return s.canceledMeanwhile(inv, 0)
	} else if err != nil {
		return nil, fmt.Errorf("mark subscription %s past due: %w", s.sub.Id(), err)
	}
	return &Step{Action: declined, Invoice: inv, Reason: res.Error}, nil
}

// escalate hands a payment attempt whose outcome is still not known — once the
// retry schedule is spent, or when the subscription no longer has what would
// repeat it — to an operator: the invoice keeps the attempt, and the
// subscription is unpaid, which confers no tier. It is logged as an alert the
// first time. The cycle keeps settling an unpaid row: a paid invoice moves it
// onto its period and back to active, a voided one ends it.
func (s *settlement) escalate(inv *billinginvoice.BillingInvoice, reason string) (*Step, error) {
	if s.sub.Status == subscription.Unpaid {
		if !s.run.DryRun {
			if err := inv.Update(); err != nil {
				return nil, fmt.Errorf("record the unresolved attempt on invoice %s: %w", inv.Id(), err)
			}
		}
		return &Step{Action: Skipped, Invoice: inv, Reason: "escalated; the attempt still has no known outcome: " + reason}, nil
	}
	if !s.run.DryRun {
		log.Error("billing cycle: ALERT subscription %s: the payment attempt %s on invoice %s has no known outcome; "+
			"the subscription is unpaid until the processor answers or an operator voids the invoice: %s", s.sub.Id(), inv.PendingKey, inv.Id(), reason)
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("record the unresolved attempt on invoice %s: %w", inv.Id(), err)
		}
	}
	if err := s.save(func(row *subscription.Subscription) { row.Status = subscription.Unpaid }); errors.Is(err, errCanceled) {
		return s.canceledMeanwhile(inv, 0)
	} else if err != nil {
		return nil, fmt.Errorf("mark subscription %s unpaid: %w", s.sub.Id(), err)
	}
	return &Step{Action: Escalated, Invoice: inv, Reason: reason}, nil
}

// comp moves a comped subscription on to the period holding now, charging
// nothing, and voids a renewal invoice left open for it, which nobody collects.
func (s *settlement) comp(inv *billinginvoice.BillingInvoice) (*Step, error) {
	if err := s.voidInvoice(inv); err != nil {
		return nil, err
	}
	start, end := s.sub.PeriodStart, s.sub.PeriodEnd
	for !s.now.Before(end) {
		start, end = end, Advance(end, &s.sub.Plan)
	}
	if err := s.save(func(row *subscription.Subscription) {
		row.PeriodStart, row.PeriodEnd, row.Status = start, end, subscription.Active
	}); errors.Is(err, errCanceled) {
		return s.canceledMeanwhile(nil, 0)
	} else if err != nil {
		return nil, fmt.Errorf("move comped subscription %s on: %w", s.sub.Id(), err)
	}
	return &Step{Action: Comped, Invoice: inv, Reason: s.c.Reason}, nil
}

// advance moves the subscription onto the period inv paid for. An invoice paid
// only once its period was over buys a whole period from the moment it was paid,
// so a customer is never charged for a period and served none of it.
func (s *settlement) advance(inv *billinginvoice.BillingInvoice, action Action, charged int64) (*Step, error) {
	start, end := s.start, s.end
	if inv.Status == billinginvoice.Paid && !inv.PaidAt.IsZero() && !inv.PaidAt.Before(end) {
		start, end = inv.PaidAt, Advance(inv.PaidAt, &s.sub.Plan)
	}
	dryRun := s.run.DryRun
	if err := s.save(func(row *subscription.Subscription) {
		row.PeriodStart, row.PeriodEnd, row.Status = start, end, subscription.Active
		if !dryRun {
			row.CurrentInvoiceId = inv.Id()
		}
	}); errors.Is(err, errCanceled) {
		return s.canceledMeanwhile(inv, charged)
	} else if err != nil {
		return nil, fmt.Errorf("move subscription %s onto its paid period: %w", s.sub.Id(), err)
	}
	return &Step{Action: action, Invoice: inv, AmountCharged: charged}, nil
}

// void ends the subscription at the end of its paid period, voiding the renewal
// invoice when one was issued, so nothing is ever collected on it.
func (s *settlement) void(inv *billinginvoice.BillingInvoice, action Action, reason string) (*Step, error) {
	if inv != nil && inv.PendingKey != "" {
		// Its last payment attempt may have moved money: the invoice is never voided
		// under it. Try it once more, and escalate if it still cannot be known.
		return s.attempt(inv, Retried, RetryFailed)
	}
	if err := s.voidInvoice(inv); err != nil {
		return nil, err
	}
	return s.close(action, inv, s.sub.PeriodEnd, reason)
}

// voidStale voids an open renewal invoice no payment was attempted on that
// bills a period the subscription no longer renews onto (its plan's interval
// changed after it was issued), under its lock; one another path holds is left
// for the next run.
func (s *settlement) voidStale(old *billinginvoice.BillingInvoice) error {
	release, err := LockInvoice(s.db, old.Id())
	if errors.Is(err, ErrInvoiceBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer release()
	inv, err := loadInvoice(s.db, old.Id())
	if err != nil {
		return fmt.Errorf("read stale renewal invoice %s: %w", old.Id(), err)
	}
	if inv.Status != billinginvoice.Open || attempted(inv) {
		return nil
	}
	return s.voidInvoice(inv)
}

// voidInvoice voids an open renewal invoice nobody will pay, giving back what
// it collected toward a period that is not served (ReturnPaid).
func (s *settlement) voidInvoice(inv *billinginvoice.BillingInvoice) error {
	if inv == nil || s.run.DryRun {
		return nil
	}
	if err := inv.MarkVoid(); err != nil {
		return err
	}
	inv.VoidedAt = s.now
	if err := ReturnPaid(s.ctx, s.db, inv, s.run.Prepaid, s.now); err != nil {
		return err
	}
	if err := inv.Update(); err != nil {
		return fmt.Errorf("void renewal invoice %s: %w", inv.Id(), err)
	}
	return nil
}

// close ends the subscription at `at` and persists it.
func (s *settlement) close(action Action, inv *billinginvoice.BillingInvoice, at time.Time, reason string) (*Step, error) {
	if err := s.save(func(row *subscription.Subscription) { End(row, at, s.now, action) }); errors.Is(err, errCanceled) {
		return s.canceledMeanwhile(inv, 0)
	} else if err != nil {
		return nil, fmt.Errorf("end subscription %s: %w", s.sub.Id(), err)
	}
	return &Step{Action: action, Invoice: inv, Reason: reason}, nil
}

// save applies the fields Settle owns to a fresh read of the stored row, which
// it writes, and to the row in hand. A change another writer made while Settle
// held its copy — a cancel at period end, a new card, a plan change — is kept
// rather than overwritten by a stale whole-row write. A row canceled meanwhile
// is never moved out of Canceled: nothing is written and the answer is
// errCanceled. A dry run applies to the row in hand only.
func (s *settlement) save(apply func(*subscription.Subscription)) error {
	return s.saveWhen(nil, apply)
}

// saveWhen is save, made only when the stored row still satisfies when; a row
// another writer moved meanwhile is left as it is, and the answer is errMoved.
func (s *settlement) saveWhen(when func(*subscription.Subscription) bool, apply func(*subscription.Subscription)) error {
	if s.run.DryRun {
		apply(s.sub)
		return nil
	}
	return saveWhen(s.db, s.sub, when, apply)
}

// errCanceled is a subscription the customer canceled while a step was under
// way on it.
var errCanceled = errors.New("the subscription was canceled meanwhile")

// errMoved is a subscription another writer moved while a step was under way on
// it.
var errMoved = errors.New("the subscription was moved meanwhile")

// save writes apply onto a fresh read of sub's stored row (see settlement.save).
func save(db *datastore.Datastore, sub *subscription.Subscription, apply func(*subscription.Subscription)) error {
	return saveWhen(db, sub, nil, apply)
}

// saveWhen writes apply onto a fresh read of sub's stored row when that row
// satisfies when (nil: always).
func saveWhen(db *datastore.Datastore, sub *subscription.Subscription, when func(*subscription.Subscription) bool, apply func(*subscription.Subscription)) error {
	row := subscription.New(db)
	if err := row.GetById(sub.Id()); err != nil {
		return err
	}
	if row.Status == subscription.Canceled {
		sub.Status, sub.Canceled, sub.CanceledAt, sub.Ended, sub.EndCancel = row.Status, row.Canceled, row.CanceledAt, row.Ended, row.EndCancel
		return errCanceled
	}
	if when != nil && !when(row) {
		return errMoved
	}
	apply(row)
	if err := row.Update(); err != nil {
		return err
	}
	apply(sub)
	sub.EndCancel, sub.Canceled, sub.CanceledAt = row.EndCancel, row.Canceled, row.CanceledAt
	return nil
}

// canceledMeanwhile answers a subscription the customer canceled while this
// step was under way. The row stays canceled. A renewal invoice with no attempt
// in flight is voided. One whose attempt has no known outcome keeps it, and a
// payment that landed for a period the customer will not be served is kept on
// its invoice; both are logged for an operator to reconcile or refund.
func (s *settlement) canceledMeanwhile(inv *billinginvoice.BillingInvoice, charged int64) (*Step, error) {
	switch {
	case inv != nil && inv.Status == billinginvoice.Paid && charged > 0:
		log.Error("billing cycle: ALERT subscription %s was canceled while its renewal was charged; invoice %s collected %d cents "+
			"for a period it will not be served — refund it", s.sub.Id(), inv.Id(), charged)
		return &Step{Action: ChargedAfterCancel, Invoice: inv, AmountCharged: charged,
			Reason: "canceled while its renewal was charged; refund invoice " + inv.Id()}, nil
	case inv != nil && inv.Status == billinginvoice.Open && inv.PendingKey != "":
		log.Error("billing cycle: ALERT subscription %s was canceled while the payment attempt %s on invoice %s had no known outcome; "+
			"reconcile it with the processor", s.sub.Id(), inv.PendingKey, inv.Id())
	case inv != nil && inv.Status == billinginvoice.Open:
		if err := s.voidInvoice(inv); err != nil {
			return nil, err
		}
	}
	return &Step{Action: Skipped, Invoice: inv, Reason: "canceled while its renewal was being settled"}, nil
}

// pastDue marks a row past due.
func pastDue(row *subscription.Subscription) { row.Status = subscription.PastDue }

// attempted reports whether a payment was ever attempted on inv.
func attempted(inv *billinginvoice.BillingInvoice) bool {
	return inv.AttemptCount > 0 || inv.PendingKey != "" || !inv.LastAttemptAt.IsZero()
}

// nextRetry is the first point of RetrySchedule, counted from the first decline,
// that is still after now; zero when the schedule is spent.
func nextRetry(firstDecline, now time.Time) time.Time {
	for _, d := range RetrySchedule {
		if at := firstDecline.Add(d); at.After(now) {
			return at
		}
	}
	return time.Time{}
}
