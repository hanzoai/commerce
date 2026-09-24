package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
)

// The report's actions for a live subscription the cycle took no step on.
const (
	// notDue: its paid period is still running.
	notDue engine.Action = "not_due"
	// renewedOutside: a processor outside Hanzo renews it; nothing here charges it.
	renewedOutside engine.Action = "external"
	// realignPending: it sits a period ahead of its paid invoice
	// (awaitingRealign). The realignment moves it back; until it runs, no live
	// cycle runs in its org (ErrRealignPending).
	realignPending engine.Action = "realign_pending"
)

// CycleResult is what one run of the subscription cycle did to one
// subscription, or in a dry run would do.
type CycleResult struct {
	Org            string `json:"org"`
	SubscriptionId string `json:"subscriptionId"`
	UserId         string `json:"userId"`
	Plan           string `json:"plan"`
	// Interval is the period the subscription was bought on: month or year, and
	// how many of them (sub.Plan, the plan as bought).
	Interval      string `json:"interval"`
	IntervalCount int    `json:"intervalCount"`
	// PeriodStart and PeriodEnd are the period the subscription had paid for
	// when the run found it.
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	// BillStart and BillEnd are the period the action invoiced, retried or ended
	// on, when there is one.
	BillStart time.Time `json:"billStart,omitzero"`
	BillEnd   time.Time `json:"billEnd,omitzero"`
	// PriceCents is what one period costs as bought: the plan's price times its
	// seats, before any promo.
	PriceCents int64  `json:"priceCents"`
	Currency   string `json:"currency"`
	// AmountCents is what was collected, or in a dry run what would be.
	AmountCents int64 `json:"amountCents"`
	// Source is what renews the subscription, from how it was bought: card,
	// prepaid, external, gift, comped or none.
	Source    string        `json:"source,omitempty"`
	Action    engine.Action `json:"action"`
	InvoiceId string        `json:"invoiceId,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// CycleAllotments counts the month's included-credit grants a run made, or in
// a dry run would make.
type CycleAllotments struct {
	Granted int `json:"granted"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// CycleReport is one run of the subscription cycle. Results lists every live
// subscription the run considered, in the order it took them, with what it did
// to each: renewed, retried, ended, left overdue, not yet due. Actions counts
// Results by action, and ChargedCents sums what was collected: in a dry run,
// what would be.
type CycleReport struct {
	Now          time.Time             `json:"now"`
	DryRun       bool                  `json:"dryRun"`
	Orgs         int                   `json:"orgs"`
	ChargedCents int64                 `json:"chargedCents"`
	Actions      map[engine.Action]int `json:"actions"`
	Results      []CycleResult         `json:"results"`
	Allotments   CycleAllotments       `json:"allotments"`
	Errors       []string              `json:"errors,omitempty"`
}

func newCycleReport(now time.Time, dryRun bool) *CycleReport {
	return &CycleReport{Now: now, DryRun: dryRun, Actions: map[engine.Action]int{}, Results: make([]CycleResult, 0)}
}

func (r *CycleReport) add(res CycleResult) {
	r.Results = append(r.Results, res)
	if res.Action != "" {
		r.Actions[res.Action]++
	}
	r.ChargedCents += res.AmountCents
}

// RunSubscriptionCycle is the billing cycle, for every organization, at now: it
// renews each subscription whose paid period has ended from the source it was
// bought from (renewalOf: its card, its prepaid money, never for a comped plan),
// retries declined renewals on engine.RetrySchedule and ends the subscription
// after the last, ends subscriptions canceled at period end or with nothing on
// file to pay them, leaves an overdue one as it is until its customer acts, ends
// seat and bundle rows whose paying subscription ended, and grants each
// subscriber the month's included allotment. It is the one entry point for the
// platform scheduler, and the HTTP endpoints call it too.
//
// It takes values because its caller is a schedule, not a person. Each org's
// payment credentials are read from KMS the first time one of its renewals
// charges a card, and the org's mode picks the Square sandbox or production.
// With dryRun every decision is made and reported, and nothing is charged,
// written or emitted; a card charge is reported as if the card accepted it, and a
// prepaid payment as the money stands.
//
// A live run skips an org, before anything in it is charged, while one of its
// subscriptions still sits a period ahead of its paid invoice (awaitingRealign):
// each such subscription is reported as realign_pending and the org's refusal
// (ErrRealignPending) is its entry in Errors. A dry run reports each such
// subscription the same way and settles the rest. A failure in one org is that
// org's entry in Errors; the run continues. Only failing to list the
// organizations ends it.
func RunSubscriptionCycle(ctx context.Context, kmsClient *kms.CachedClient, ev *events.Client, now time.Time, dryRun bool) (*CycleReport, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		return nil, err
	}
	report := newCycleReport(now, dryRun)
	report.Orgs = len(orgs)
	for _, org := range orgs {
		db := datastore.New(org.Namespaced(ctx))
		cycleOrg(ctx, org, db, now, dryRun, ev, cardFor(kmsClient, org), "", report)
	}
	return report, nil
}

// ErrRealignPending is an org's live cycle refused because its subscriptions
// still sit a period ahead of their paid invoice (awaitingRealign): renewed by
// the dates they hold, each would be served that period without paying for it.
// The org's realignment (POST /v1/billing/realign/run?dryRun=false) runs first.
var ErrRealignPending = errors.New("subscriptions sit a period ahead of their paid invoice; run the realignment before a live cycle")

// refuseUnrealigned answers ErrRealignPending, naming how many, when a live
// subscription in orgs — one subscriber's, when user is set — awaits
// realignment.
func refuseUnrealigned(ctx context.Context, orgs []*organization.Organization, user string) error {
	n := 0
	for _, org := range orgs {
		db := datastore.New(org.Namespaced(ctx))
		ahead, _, err := unrealigned(org, db, user)
		if err != nil {
			return fmt.Errorf("%q: %w", org.Name, err)
		}
		n += len(ahead)
	}
	if n > 0 {
		return fmt.Errorf("%w (%d)", ErrRealignPending, n)
	}
	return nil
}

// unrealigned lists the subscriptions in org — one subscriber's, when user is
// set — that await realignment, and answers the org's realignment cutover.
func unrealigned(org *organization.Organization, db *datastore.Datastore, user string) ([]*subscription.Subscription, time.Time, error) {
	cutover, err := realignCutover(db)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read the realignment's cutover: %w", err)
	}
	subs, err := orgSubscriptions(db, user)
	if err != nil {
		return nil, cutover, fmt.Errorf("list subscriptions: %w", err)
	}
	ahead := make([]*subscription.Subscription, 0)
	for _, s := range subs {
		pending, err := awaitingRealign(org, db, s, cutover)
		if err != nil {
			return nil, cutover, fmt.Errorf("subscription %s: %w", s.Id(), err)
		}
		if pending {
			ahead = append(ahead, s)
		}
	}
	return ahead, cutover, nil
}

// cycleFault is the HTTP answer to a cycle that could not start.
func cycleFault(c *zip.Ctx, err error) error {
	if errors.Is(err, ErrRealignPending) {
		return http.Fail(c, 409, err.Error(), nil)
	}
	log.Error("billing cycle: %v", err, c)
	return http.Fail(c, 500, "failed to start the billing cycle", err)
}

// cycleLocks serializes runs of one org's cycle within the process, and a
// manual renew with them: overlapping runs take turns, and each reads the
// subscriptions the one before it left.
var cycleLocks sync.Map

func lockOrgCycle(org string) func() {
	mu, _ := cycleLocks.LoadOrStore(org, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// orgCycleBudget bounds how long one org's cycle starts new subscriptions, on
// the wall clock, so an org whose processor answers slowly cannot hold back the
// orgs after it. A subscription under way is settled to the end; the ones not
// yet started are settled by the next run, and the report carries the stop as
// an error.
var orgCycleBudget = 10 * time.Minute

// cycleOrg runs the cycle for one org into report. user, when set, narrows the
// run to that subscriber.
func cycleOrg(ctx context.Context, org *organization.Organization, db *datastore.Datastore, now time.Time, dryRun bool, ev *events.Client, charge rail, user string, report *CycleReport) {
	defer lockOrgCycle(org.Name)()

	fail := func(what string, err error) {
		log.Error("billing cycle: %s for %q: %v", what, org.Name, err)
		report.Errors = append(report.Errors, fmt.Sprintf("%s: %s: %v", org.Name, what, err))
	}
	// Realignment comes first. A live run in an org with subscriptions awaiting it
	// settles nothing there; one in an org with none records that the org's
	// realignment is done, so no invoice issued from here on makes a candidate.
	ahead, cutover, err := unrealigned(org, db, user)
	if err != nil {
		fail("check realignment", err)
		return
	}
	pending := make(map[string]bool, len(ahead))
	for _, s := range ahead {
		pending[s.Id()] = true
		res := resultOf(org, s)
		res.Action, res.Reason = realignPending, "a period ahead of its paid invoice; the realignment moves it back before a live run"
		report.add(res)
	}
	switch {
	case dryRun:
	case len(ahead) > 0:
		fail("live cycle refused", fmt.Errorf("%w (%d)", ErrRealignPending, len(ahead)))
		return
	case user == "" && cutover.IsZero():
		if err := markRealigned(db, time.Now()); err != nil {
			fail("record the realignment's cutover", err)
			return
		}
	}

	run := engine.Run{Now: now, DryRun: dryRun, Prepaid: prepaidFor(ctx, org)}
	subs, err := orgSubscriptions(db, user)
	if err != nil {
		fail("list subscriptions", err)
		return
	}

	// Paying subscriptions first, so a seat or bundle row sees its parent in the
	// same run. Each settled row replaces its listed one, so what follows — the
	// seat rows and the allotment grants — reads this run's outcome, in a dry run
	// as in a real one.
	settled := make(map[string]*subscription.Subscription)
	started := time.Now()
	for i, s := range subs {
		if isBundleRow(s) || !engine.Live(s) || pending[s.Id()] {
			continue
		}
		if spent := time.Since(started); spent > orgCycleBudget {
			fail("settle subscriptions", fmt.Errorf("stopped after %s; the subscriptions not yet settled are left for the next run", spent.Round(time.Millisecond)))
			break
		}
		sub := subscription.New(db)
		if err := sub.GetById(s.Id()); err != nil {
			res := resultOf(org, s)
			res.Error = err.Error()
			report.add(res)
			continue
		}
		report.add(settleOne(ctx, org, db, sub, run, ev, charge))
		settled[sub.Id()] = sub
		subs[i] = sub
	}
	parents := make(map[string]*subscription.Subscription, len(settled))
	for id, sub := range settled {
		parents[id] = sub
	}
	for i, s := range subs {
		if !isBundleRow(s) || !grantsTier(s.Status) {
			continue
		}
		parent := parentOf(db, bundleParentOf(s), parents)
		switch {
		case parent == nil:
		case parent.Status == subscription.Canceled:
			report.add(endWithParent(ctx, org, db, s, now, dryRun, ev))
		default:
			if err := followParent(db, s, parent, dryRun); err != nil {
				fail("move seat "+s.Id()+" onto its paying subscription's period", err)
			}
			subs[i] = s
		}
	}
	for _, res := range resolveCanceled(ctx, org, db, subs, settled, run, charge) {
		report.add(res)
	}

	granted, skipped, results := grantOrgAllotments(db, subs, now, !org.TestMode(), dryRun)
	report.Allotments.Granted += granted
	report.Allotments.Skipped += skipped
	for _, r := range results {
		if _, failed := r["error"]; failed {
			report.Allotments.Failed++
		}
	}
}

// followParent moves a seat or bundle row onto its paying subscription's
// period, so it is served, and granted its month, for exactly the time the
// payment covers. A dry run moves the row in hand only.
func followParent(db *datastore.Datastore, s, parent *subscription.Subscription, dryRun bool) error {
	if s.PeriodStart.Equal(parent.PeriodStart) && s.PeriodEnd.Equal(parent.PeriodEnd) {
		return nil
	}
	s.PeriodStart, s.PeriodEnd = parent.PeriodStart, parent.PeriodEnd
	if dryRun {
		return nil
	}
	row := subscription.New(db)
	if err := row.GetById(s.Id()); err != nil {
		return err
	}
	if row.Status == subscription.Canceled {
		s.Status = row.Status
		return nil
	}
	row.PeriodStart, row.PeriodEnd = parent.PeriodStart, parent.PeriodEnd
	return row.Update()
}

// resolveCanceled settles what canceled subscriptions' invoices still hold
// (engine.ResolveCanceled): a payment attempt of unknown outcome, looked up or
// given back through the source its subscription was bought from, and money a
// replaced subscription's invoice collected toward a period it is never served.
// It answers a report line for each. A subscription this run already settled is
// left for the next run.
func resolveCanceled(ctx context.Context, org *organization.Organization, db *datastore.Datastore, subs []*subscription.Subscription, settled map[string]*subscription.Subscription, run engine.Run, charge rail) []CycleResult {
	open := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("Status=", billinginvoice.Open).GetAll(&open); err != nil {
		return []CycleResult{{Org: org.Name, Error: "list open invoices: " + err.Error()}}
	}
	pending := make(map[string]bool)
	for _, inv := range open {
		if inv.PendingKey != "" && inv.SubscriptionId != "" {
			pending[inv.SubscriptionId] = true
		}
	}
	out := make([]CycleResult, 0)
	for _, s := range subs {
		replaced, _ := s.Metadata["endReason"].(string)
		if _, now := settled[s.Id()]; now || s.Status != subscription.Canceled || (!pending[s.Id()] && replaced != string(engine.Replaced)) {
			continue
		}
		res := resultOf(org, s)
		c, source, err := renewalOf(org, db, s, charge, run.Prepaid)
		if err != nil {
			res.Error = err.Error()
			out = append(out, res)
			continue
		}
		res.Source = source
		step, err := engine.ResolveCanceled(ctx, db, s, run, c)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Action, res.AmountCents, res.Reason = step.Action, step.AmountCharged, step.Reason
			if inv := step.Invoice; inv != nil {
				res.BillStart, res.BillEnd = inv.PeriodStart, inv.PeriodEnd
				if !run.DryRun {
					res.InvoiceId = inv.Id()
				}
			}
		}
		if res.Action != "" || res.Error != "" {
			out = append(out, res)
		}
	}
	return out
}

// resultOf is the report line for sub before a step is taken on it.
func resultOf(org *organization.Organization, sub *subscription.Subscription) CycleResult {
	count := sub.Plan.IntervalCount
	if count <= 0 {
		count = 1
	}
	cur := string(sub.Plan.Currency)
	if cur == "" {
		cur = "usd"
	}
	return CycleResult{
		Org: org.Name, SubscriptionId: sub.Id(), UserId: sub.UserId, Plan: planOf(sub),
		Interval: string(sub.Plan.Interval), IntervalCount: count,
		PeriodStart: sub.PeriodStart, PeriodEnd: sub.PeriodEnd,
		PriceCents: int64(sub.Plan.Price) * seatCount(sub), Currency: cur,
	}
}

// seatCount is the number of seats a subscription's price is multiplied by.
func seatCount(sub *subscription.Subscription) int64 {
	if !sub.Plan.PerSeat || sub.Quantity < 1 {
		return 1
	}
	return int64(sub.Quantity)
}

// settleOne takes the cycle's step for one subscription, bound to db: the
// engine's decision (engine.Settle), after the rules only this package knows —
// the org's mode, who bills the subscription, and how it was bought (renewalOf).
func settleOne(ctx context.Context, org *organization.Organization, db *datastore.Datastore, sub *subscription.Subscription, run engine.Run, ev *events.Client, charge rail) CycleResult {
	dryRun := run.DryRun
	res := resultOf(org, sub)
	if !engine.Live(sub) {
		return res
	}
	// An outside processor that bills a subscription renews it, and its webhooks
	// keep the row's status: it is not the cycle's.
	if billedElsewhere(sub) {
		res.Source, res.Action = sourceExternal, renewedOutside
		res.Reason = "billed by " + sub.ProviderType + ", which renews it"
		return res
	}
	c, source, err := renewalOf(org, db, sub, charge, run.Prepaid)
	if err != nil {
		log.Error("billing cycle: how subscription %s in %q is paid: %v", sub.Id(), org.Name, err)
		res.Error = err.Error()
		return res
	}
	if sub.Test != org.TestMode() {
		// Bought in the org's other mode: nothing on this org's processor or books
		// pays it, and a live row would otherwise keep serving its tier to every
		// reader.
		c = engine.Collection{End: engine.EndedOtherMode,
			Reason: "bought in the org's other mode (test " + strconv.FormatBool(sub.Test) + "); nothing on this org's processor pays it"}
		source = sourceNone
	}
	res.Source = source
	if source == sourceExternal {
		res.Action, res.Reason = renewedOutside, c.Reason
		return res
	}

	step, err := engine.Settle(ctx, db, sub, run, c)
	if err != nil {
		log.Error("billing cycle: settle subscription %s in %q: %v", sub.Id(), org.Name, err)
		res.Error = err.Error()
		return res
	}
	res.Action, res.AmountCents, res.Reason = step.Action, step.AmountCharged, step.Reason
	if res.Action == "" {
		res.Action = notDue
	}
	if inv := step.Invoice; inv != nil {
		res.BillStart, res.BillEnd = inv.PeriodStart, inv.PeriodEnd
		if !dryRun {
			res.InvoiceId = inv.Id()
		}
	}
	if !dryRun {
		emitStep(ctx, ev, org.Name, sub, step)
		if engine.Live(sub) && engine.Lapsed(sub, run.Now) {
			emitLapsed(ctx, ev, db, org.Name, sub)
		}
	}
	return res
}

// emitLapsed fires subscription_lapsed for sub once per paid period it lapsed
// past: the first run to see it lapsed takes the guard, keyed by the period
// end, and every later run finds it taken. A nil collector is a no-op and takes
// no guard.
func emitLapsed(ctx context.Context, ev *events.Client, db *datastore.Datastore, orgName string, sub *subscription.Subscription) {
	if ev == nil {
		return
	}
	rec, replay, err := idempotencykey.Begin(db, "billing-lapsed:"+sub.Id(), "end:"+strconv.FormatInt(sub.PeriodEnd.Unix(), 10))
	if err != nil || replay {
		return
	}
	_ = idempotencykey.Complete(rec, "emitted")
	e := subscriptionEvent(orgName, sub)
	e.Status = "lapsed"
	go ev.EmitSubscriptionLapsed(context.WithoutCancel(ctx), e)
}

// endWithParent ends a seat or bundle row whose paying subscription has ended.
// It charges nothing: the row never carried a price of its own.
func endWithParent(ctx context.Context, org *organization.Organization, db *datastore.Datastore, s *subscription.Subscription, now time.Time, dryRun bool, ev *events.Client) CycleResult {
	res := resultOf(org, s)
	res.Action, res.Reason = engine.EndedWithParent, "its paying subscription "+bundleParentOf(s)+" ended"
	engine.End(s, now, now, engine.EndedWithParent) // the listed row, which the allotment grants read next
	if dryRun {
		return res
	}
	sub := subscription.New(db)
	if err := sub.GetById(s.Id()); err != nil {
		res.Error = err.Error()
		return res
	}
	if sub.Status == subscription.Canceled {
		res.Action, res.Reason = engine.Skipped, "canceled meanwhile"
		return res
	}
	engine.End(sub, now, now, engine.EndedWithParent)
	if err := sub.Update(); err != nil {
		res.Error = err.Error()
		return res
	}
	emitStep(ctx, ev, org.Name, sub, &engine.Step{Action: engine.EndedWithParent})
	return res
}

// emitStep fires the analytics events a settled step produces, on the events
// the collector names: a renewal is subscription_renewed (and invoice_paid when
// the step collected), and every end — whatever ended it — is
// subscription_canceled carrying the action as its reason (and invoice_void when
// it voided the renewal invoice), so a notification can hang off an end. A
// payment that landed after the customer canceled is invoice_paid alone; the
// cancel emitted its own event. A declined attempt, a comped period and a
// skipped row have no event name and emit nothing.
// Fire-and-forget: a nil collector is a no-op.
func emitStep(ctx context.Context, ev *events.Client, orgName string, sub *subscription.Subscription, step *engine.Step) {
	if ev == nil {
		return
	}
	detached := context.WithoutCancel(ctx)
	switch {
	case step.Action == engine.Renewed || step.Action == engine.Retried:
		go ev.EmitSubscriptionRenewed(detached, subscriptionEvent(orgName, sub))
		if step.Invoice != nil && step.AmountCharged > 0 {
			go ev.EmitInvoicePaid(detached, invoiceEvent(orgName, step.Invoice))
		}
	case step.Action == engine.ChargedAfterCancel:
		go ev.EmitInvoicePaid(detached, invoiceEvent(orgName, step.Invoice))
	case sub.Status == subscription.Canceled && step.Action != engine.Skipped:
		e := subscriptionEvent(orgName, sub)
		e.Reason = string(step.Action)
		go ev.EmitSubscriptionCanceled(detached, e)
		if step.Invoice != nil && step.Invoice.Status == billinginvoice.Void {
			go ev.EmitInvoiceVoid(detached, invoiceEvent(orgName, step.Invoice))
		}
	}
}

// orgSubscriptions lists the org's subscriptions, or one subscriber's.
// Subscriptions are stored without an ancestor (see userSubscriptions), so they
// are read by namespace alone; the namespace is the tenant boundary.
func orgSubscriptions(db *datastore.Datastore, user string) ([]*subscription.Subscription, error) {
	q := subscription.Query(db)
	if user != "" {
		q = q.Filter("UserId=", user)
	}
	subs := make([]*subscription.Subscription, 0)
	if _, err := q.GetAll(&subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// rail is an org's saved-card processor as renewals reach it: charge a card,
// and look up what became of a charge whose answer was lost.
type rail struct {
	charge engine.ProviderCharger
	look   engine.Looker
}

// railOf is org's rail with its payment credentials read as they stand.
func railOf(org *organization.Organization) rail {
	return rail{charge: chargeProviderForOrg(org), look: lookProviderForOrg(org)}
}

// cardFor is org's renewal rail. It reads the org's payment credentials from
// KMS the first time a renewal charges a card or looks a charge up, so a run
// that touches no card in an org never reads its secrets. When that read fails,
// no card is charged or looked up at all: the processor would fall back to the
// deployment's own credentials, which are not the org's. Every charge then
// answers engine.ErrChargeUnknown, so it is repeated on a later run rather than
// counted against the customer, every look-up answers not known, and the
// failure is logged for an operator.
func cardFor(kmsClient *kms.CachedClient, org *organization.Organization) rail {
	r := railOf(org)
	var once sync.Once
	var hydrateErr error
	hydrate := func() error {
		once.Do(func() {
			if kmsClient == nil {
				return
			}
			if hydrateErr = kms.Hydrate(kmsClient, org); hydrateErr != nil {
				log.Error("billing: ALERT the payment credentials of org %q could not be read from KMS; no card in it is charged until they can: %v", org.Name, hydrateErr)
			}
		})
		return hydrateErr
	}
	return rail{
		charge: func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amountCents int64) (string, error) {
			if err := hydrate(); err != nil {
				return "", fmt.Errorf("%w: the org's payment credentials could not be read from KMS: %v", engine.ErrChargeUnknown, err)
			}
			return r.charge(ctx, db, inv, amountCents)
		},
		look: func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice) (engine.Looked, error) {
			if err := hydrate(); err != nil {
				return engine.Looked{}, fmt.Errorf("the org's payment credentials could not be read from KMS: %w", err)
			}
			return r.look(ctx, db, inv)
		},
	}
}

// isBundleRow reports whether s is a seat or bundle row: an entitlement another
// subscription pays for, never billed itself.
func isBundleRow(s *subscription.Subscription) bool {
	return strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle")
}

// bundleParentOf is the subscription a seat or bundle row belongs to, from the
// link createSubscription and provisionMembers store on it; "" for a row stored
// before subscription metadata was kept.
func bundleParentOf(s *subscription.Subscription) string {
	id, _ := s.Metadata["bundleParent"].(string)
	return strings.TrimSpace(id)
}

// parentOf is the paying subscription id, as this run left it or, for one the
// run did not settle, as stored (and then kept in parents); nil when there is
// none.
func parentOf(db *datastore.Datastore, id string, parents map[string]*subscription.Subscription) *subscription.Subscription {
	if id == "" {
		return nil
	}
	if p, ok := parents[id]; ok {
		return p
	}
	parent := subscription.New(db)
	if err := parent.GetById(id); err != nil {
		return nil
	}
	parents[id] = parent
	return parent
}

// grantsTier reports whether a subscription in this status still confers its
// plan.
func grantsTier(s subscription.Status) bool {
	switch s {
	case subscription.Active, subscription.Trialing, subscription.PastDue:
		return true
	}
	return false
}

// billedElsewhere reports whether an outside processor runs the subscription's
// billing, so the cycle must never charge it: the processor renews it and
// charges the customer itself.
func billedElsewhere(s *subscription.Subscription) bool {
	if s.Type == subscription.External {
		return false // a payment recorded by hand: renewalOf answers it
	}
	switch strings.ToLower(strings.TrimSpace(s.ProviderType)) {
	case "stripe", "paypal", "authorizenet", "authorize.net", "braintree":
		return true
	}
	return false
}

// planOf is the plan slug a subscription holds.
func planOf(s *subscription.Subscription) string {
	if s.Plan.Slug != "" {
		return s.Plan.Slug
	}
	return s.PlanId
}

// dryRunOf reads ?dryRun= off a request to the cycle or the realignment. A run
// is a dry run unless the request says dryRun=false: the live run is always a
// deliberate, separate invocation. Any other query parameter (a misspelled
// dryrun, dry_run) or a dryRun that is not true or false is an error, and the
// endpoint refuses rather than guessing.
func dryRunOf(c *zip.Ctx) (bool, error) {
	for name := range c.Fiber().Queries() {
		if name != "dryRun" {
			return true, fmt.Errorf("unknown query parameter %q; this endpoint takes only dryRun", name)
		}
	}
	raw, ok := c.Fiber().Queries()["dryRun"]
	if !ok {
		return true, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return true, fmt.Errorf("dryRun must be true or false, not %q", raw)
	}
	return v, nil
}

// RunBillingCycle runs the subscription cycle for the request's organization.
// It is a dry run unless the request says dryRun=false, and a live run is a 409
// while a subscription awaits realignment (ErrRealignPending). The run is not
// tied to the request: a caller that stops waiting does not stop a run that may
// already have charged a card.
//
//	POST /v1/billing/cycle/run[?dryRun=false]
func RunBillingCycle(c *zip.Ctx) error {
	ctx := context.WithoutCancel(c.Context())
	org := middleware.GetOrganization(c)
	dryRun, err := dryRunOf(c)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}
	if !dryRun {
		if err := refuseUnrealigned(ctx, []*organization.Organization{org}, ""); err != nil {
			return cycleFault(c, err)
		}
	}
	report := newCycleReport(time.Now(), dryRun)
	report.Orgs = 1
	db := datastore.New(org.Namespaced(ctx))
	cycleOrg(ctx, org, db, report.Now, report.DryRun, eventsOf(c), cardFor(kmsOf(c), org), "", report)
	return c.JSON(200, report)
}

type cycleUserRequest struct {
	UserId string `json:"userId"`
}

// RunBillingCycleUser runs the subscription cycle for one subscriber in the
// request's organization: a dry run unless the request says dryRun=false, and
// not tied to the request (see RunBillingCycle).
//
//	POST /v1/billing/cycle/run-user[?dryRun=false]  {"userId": "..."}
func RunBillingCycleUser(c *zip.Ctx) error {
	ctx := context.WithoutCancel(c.Context())
	org := middleware.GetOrganization(c)
	dryRun, err := dryRunOf(c)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}

	var req cycleUserRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	if !dryRun {
		if err := refuseUnrealigned(ctx, []*organization.Organization{org}, req.UserId); err != nil {
			return cycleFault(c, err)
		}
	}
	report := newCycleReport(time.Now(), dryRun)
	report.Orgs = 1
	db := datastore.New(org.Namespaced(ctx))
	cycleOrg(ctx, org, db, report.Now, report.DryRun, eventsOf(c), cardFor(kmsOf(c), org), req.UserId, report)
	return c.JSON(200, report)
}

// RunBillingCycleAllOrgs runs the subscription cycle for every organization: a
// dry run unless the request says dryRun=false, and not tied to the request (see
// RunBillingCycle).
//
//	POST /v1/billing/cycle/run-all[?dryRun=false]
func RunBillingCycleAllOrgs(c *zip.Ctx) error {
	dryRun, err := dryRunOf(c)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}
	report, err := RunSubscriptionCycle(context.WithoutCancel(c.Context()), kmsOf(c), eventsOf(c), time.Now(), dryRun)
	if err != nil {
		return cycleFault(c, err)
	}
	return c.JSON(200, report)
}
