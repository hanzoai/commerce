package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	squarecore "github.com/square/square-go-sdk/v3/core"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	gift "github.com/hanzoai/commerce/billing/grant"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The subscription cycle end to end: the real subscribe path, the real Square
// charger against the in-memory Square, and time held in the test's hand.

// squareDecline is Square refusing the card: a 402 with a decline code.
func squareDecline() error {
	return squarecore.NewAPIError(402, nil, errors.New(`{"errors":[{"category":"PAYMENT_METHOD_ERROR","code":"GENERIC_DECLINE"}]}`))
}

// devAllotment gives dev an included monthly allotment of usd dollars for the
// life of one test and restores the catalog after it. The published catalog
// mints no allotment, so the grant is exercised against a fixture, the way
// maxLevels fixtures a price ladder.
func devAllotment(t *testing.T, usd int) int64 {
	t.Helper()
	p := lookupPlan("dev")
	if p == nil {
		t.Fatal("plan \"dev\" missing from the catalog")
	}
	was := p.Limits
	lims := plan.Limits{}
	if was != nil {
		lims = *was
	}
	lims.IncludedCloudCredits = &usd
	p.Limits = &lims
	t.Cleanup(func() { p.Limits = was })
	return IncludedMonthlyCents("dev")
}

// cycleAt runs one org's cycle at now and returns its report.
func cycleAt(t *testing.T, ctx context.Context, org *organization.Organization, now time.Time, dryRun bool) *CycleReport {
	t.Helper()
	report := newCycleReport(now, dryRun)
	db := datastore.New(org.Namespaced(ctx))
	cycleOrg(ctx, org, db, now, dryRun, nil, chargeProviderForOrg(org), "", report)
	if len(report.Errors) > 0 {
		t.Fatalf("cycle at %s: %v", now, report.Errors)
	}
	for _, r := range report.Results {
		if r.Error != "" {
			t.Fatalf("cycle at %s: %s: %s", now, r.SubscriptionId, r.Error)
		}
	}
	return report
}

// actionsOf is the report's actions by subscription id.
func actionsOf(r *CycleReport) map[string]engine.Action {
	out := make(map[string]engine.Action, len(r.Results))
	for _, res := range r.Results {
		out[res.SubscriptionId] = res.Action
	}
	return out
}

// only asserts the report took a step on exactly one subscription, with the
// given action.
func only(t *testing.T, r *CycleReport, want engine.Action) CycleResult {
	t.Helper()
	got := acted(r)
	if len(got) != 1 || got[0].Action != want {
		t.Fatalf("results = %+v, want exactly one %s", got, want)
	}
	return got[0]
}

// subscribeByCard buys planID through the real card endpoint and returns the
// stored subscription.
func subscribeByCard(t *testing.T, ctx context.Context, org *organization.Organization, body string) *subscription.Subscription {
	t.Helper()
	resp := invokeSubscribeCard(org, ctx, body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("subscribe status=%d, want 201", resp.StatusCode)
	}
	id, _ := jsonBody(t, resp)["subscriptionId"].(string)
	sub := subscription.New(datastore.New(org.Namespaced(ctx)))
	if err := sub.GetById(id); err != nil {
		t.Fatalf("read subscription %q: %v", id, err)
	}
	return sub
}

// cardSubThrough is seedCardBackedSub paid through `through`.
func cardSubThrough(t *testing.T, db *datastore.Datastore, subject string, through time.Time) *subscription.Subscription {
	t.Helper()
	s := seedCardBackedSub(t, db, subject, "dev", "ccof_"+subject, "cust_"+subject)
	s.PeriodEnd = through
	s.PeriodStart = through.AddDate(0, -1, 0)
	if err := s.Update(); err != nil {
		t.Fatalf("set period: %v", err)
	}
	return s
}

func reloadSub(t *testing.T, db *datastore.Datastore, id string) *subscription.Subscription {
	t.Helper()
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("reload subscription %s: %v", id, err)
	}
	return s
}

func tierOf(t *testing.T, ctx context.Context, org *organization.Organization, user string) tier.Name {
	t.Helper()
	n, err := TierOf(ctx, org, user)
	if err != nil {
		t.Fatalf("tier of %s: %v", user, err)
	}
	return n
}

// TestCycle_FindsWhatTheCardPathOpened: a subscription the card endpoint opened
// is listed by ListSubscriptions and renewed by the cycle — both read
// subscriptions the way they are stored, without an ancestor.
func TestCycle_FindsWhatTheCardPathOpened(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-found")
	m := squareMock("cust_f", "ccof_f", "sqpay_f")
	withFakeSquare(t, m)

	sub := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev"}`)

	listed, err := ListSubscriptions(ctx, org, "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, s := range listed {
		found = found || s.Id() == sub.Id()
	}
	if !found {
		t.Fatalf("ListSubscriptions did not return %s (%d rows)", sub.Id(), len(listed))
	}

	r := cycleAt(t, ctx, org, sub.PeriodEnd, false)
	if got := actionsOf(r)[sub.Id()]; got != engine.Renewed {
		t.Fatalf("cycle action for the card subscription = %q, want renewed", got)
	}
}

// renewalsFromSubscribe buys dev by the given interval and drives the cycle
// through two renewals, checking every charge lands exactly when the paid period
// ends and never a second before.
func renewalsFromSubscribe(t *testing.T, name, body string, price int64, step func(time.Time) time.Time) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg(name)
	m := squareMock("cust_"+name, "ccof_"+name, "sqpay_"+name)
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	sub := subscribeByCard(t, ctx, org, body)
	if m.chargeCalls != 1 || m.lastChargeAmount != price {
		t.Fatalf("subscribe charged %d times, last %d; want once at %d", m.chargeCalls, m.lastChargeAmount, price)
	}
	t0 := sub.PeriodStart
	if want := step(t0); !sub.PeriodEnd.Equal(want) {
		t.Fatalf("subscribed period %s..%s, want it to end %s — the first charge paid for it", t0, sub.PeriodEnd, want)
	}

	end := sub.PeriodEnd
	for renewal := 1; renewal <= 2; renewal++ {
		if r := cycleAt(t, ctx, org, end.Add(-time.Second), false); len(acted(r)) != 0 || m.chargeCalls != renewal {
			t.Fatalf("renewal %d: a second before the period ended the cycle did %+v, charges %d", renewal, acted(r), m.chargeCalls)
		}
		res := only(t, cycleAt(t, ctx, org, end, false), engine.Renewed)
		if m.chargeCalls != renewal+1 || m.lastChargeAmount != price || res.AmountCents != price {
			t.Fatalf("renewal %d: charges %d last %d reported %d, want %d charges at %d", renewal, m.chargeCalls, m.lastChargeAmount, res.AmountCents, renewal+1, price)
		}
		got := reloadSub(t, db, sub.Id())
		if !got.PeriodStart.Equal(end) || !got.PeriodEnd.Equal(step(end)) || got.Status != subscription.Active {
			t.Fatalf("renewal %d: subscription %s %s..%s, want active %s..%s", renewal, got.Status, got.PeriodStart, got.PeriodEnd, end, step(end))
		}
		end = got.PeriodEnd
	}

	invs := invoicesForSub(t, db, sub.Id())
	sort.Slice(invs, func(i, j int) bool { return invs[i].PeriodStart.Before(invs[j].PeriodStart) })
	if len(invs) != 3 {
		t.Fatalf("invoices = %d, want the subscribe and two renewals", len(invs))
	}
	for i, inv := range invs {
		if inv.Status != billinginvoice.Paid || inv.AmountDue != price || !inv.PeriodEnd.Equal(step(inv.PeriodStart)) {
			t.Fatalf("invoice %d: %s %d for %s..%s", i, inv.Status, inv.AmountDue, inv.PeriodStart, inv.PeriodEnd)
		}
		if i > 0 && !inv.PeriodStart.Equal(invs[i-1].PeriodEnd) {
			t.Fatalf("invoice %d starts %s, want it to follow the last one at %s", i, inv.PeriodStart, invs[i-1].PeriodEnd)
		}
	}
}

func TestCycle_MonthlyFromSubscribeThroughTwoRenewals(t *testing.T) {
	renewalsFromSubscribe(t, "cyc-month", `{"sourceId":"cnon:ok","planId":"dev"}`,
		lookupPlan("dev").Price, func(at time.Time) time.Time { return at.AddDate(0, 1, 0) })
}

func TestCycle_AnnualFromSubscribeThroughTwoRenewals(t *testing.T) {
	year := int64(lookupPlan("dev").AnnualTotal)
	if year <= 0 {
		t.Fatal("dev publishes no annual total")
	}
	renewalsFromSubscribe(t, "cyc-year", `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`,
		year, func(at time.Time) time.Time { return at.AddDate(1, 0, 0) })
}

// TestCycle_AnnualSubscriberIsGrantedMonthly: the included allotment is monthly
// whatever the billing interval, so an annual subscriber is granted each month
// the cycle runs, once per month.
func TestCycle_AnnualSubscriberIsGrantedMonthly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-yr-allot")
	withFakeSquare(t, squareMock("cust_ya", "ccof_ya", "sqpay_ya"))
	if devAllotment(t, 5) != 500 {
		t.Fatal("the fixture allotment did not take")
	}
	sub := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`)
	if sub.Plan.Interval != "year" {
		t.Fatalf("stored interval %q, want year", sub.Plan.Interval)
	}

	for month := 1; month <= 3; month++ {
		at := sub.PeriodStart.AddDate(0, month, 0)
		if r := cycleAt(t, ctx, org, at, false); r.Allotments.Granted != 1 || len(acted(r)) != 0 {
			t.Fatalf("month %d: granted %d results %+v, want one grant and no renewal", month, r.Allotments.Granted, acted(r))
		}
		if r := cycleAt(t, ctx, org, at.Add(time.Minute), false); r.Allotments.Granted != 0 {
			t.Fatalf("month %d: a second run granted %d again", month, r.Allotments.Granted)
		}
	}
}

// TestCycle_RunTwiceChargesOnce: repeated and overlapping runs at one instant
// charge a due subscription once.
func TestCycle_RunTwiceChargesOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-twice")
	m := squareMock("", "", "sqpay_twice")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-twice", now.Add(-time.Hour))

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report := newCycleReport(now, false)
			cycleOrg(ctx, org, datastore.New(org.Namespaced(ctx)), now, false, nil, chargeProviderForOrg(org), "", report)
		}()
	}
	wg.Wait()
	cycleAt(t, ctx, org, now, false)

	if m.chargeCalls != 1 {
		t.Fatalf("card charged %d times, want 1", m.chargeCalls)
	}
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 1 {
		t.Fatalf("invoices = %d, want 1", len(invs))
	}
}

// TestCycle_RepeatedAttemptReplaysTheCharge: a run that charged the card and
// lost its writes is repeated by the next run at the same attempt count, so the
// same idempotency key, and Square answers with the first charge instead of making
// a second.
func TestCycle_RepeatedAttemptReplaysTheCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-replay")
	m := squareMock("", "", "sqpay_replay")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-replay", now.Add(-time.Hour))
	through := sub.PeriodEnd

	only(t, cycleAt(t, ctx, org, now, false), engine.Renewed)

	// Put back the state the charge found, as if the run died before writing.
	inv := invoicesForSub(t, db, sub.Id())[0]
	lost, err := invoiceRow(db, inv.Id())
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	lost.Status, lost.AmountPaid, lost.AttemptCount = billinginvoice.Open, 0, 0
	lost.PaidAt, lost.LastAttemptAt, lost.PaymentMethod, lost.PaymentRef = time.Time{}, time.Time{}, "", ""
	if err := lost.Update(); err != nil {
		t.Fatalf("revert invoice: %v", err)
	}
	back := reloadSub(t, db, sub.Id())
	back.PeriodEnd, back.PeriodStart = through, through.AddDate(0, -1, 0)
	if err := back.Update(); err != nil {
		t.Fatalf("revert subscription: %v", err)
	}

	only(t, cycleAt(t, ctx, org, now.Add(time.Minute), false), engine.Renewed)
	if m.chargeCalls != 1 {
		t.Fatalf("Square saw %d charges, want the one — the repeat must reuse its key", m.chargeCalls)
	}
	if got := reloadSub(t, db, sub.Id()); !got.PeriodEnd.Equal(through.AddDate(0, 1, 0)) {
		t.Fatalf("subscription through %s, want the renewed month", got.PeriodEnd)
	}
}

// TestCycle_OverdueIsLeftUntilTheCustomerRenews: a subscription whose paid
// period ended three months ago with no renewal — the cycle was not running — is
// neither charged nor changed, run after run, and confers nothing. When its
// customer renews it, one period is charged, the one from that moment, and the
// months it was not renewed for are never billed.
func TestCycle_OverdueIsLeftUntilTheCustomerRenews(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-overdue")
	m := squareMock("", "", "sqpay_overdue")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	through := now.AddDate(0, -3, 0)
	sub := cardSubThrough(t, db, "cyc-overdue", through)
	before := reloadSub(t, db, sub.Id())

	for _, at := range []time.Time{now, now.Add(24 * time.Hour), now.AddDate(0, 1, 0)} {
		res := only(t, cycleAt(t, ctx, org, at, false), engine.Overdue)
		if res.AmountCents != 0 || res.Source != sourceCard || !res.PeriodEnd.Equal(through) {
			t.Fatalf("at %s: %+v, want overdue on the card, paid through %s, nothing charged", at, res, through)
		}
	}
	got := reloadSub(t, db, sub.Id())
	if m.chargeCalls != 0 || len(invoicesForSub(t, db, sub.Id())) != 0 {
		t.Fatalf("charges=%d invoices=%d, want none", m.chargeCalls, len(invoicesForSub(t, db, sub.Id())))
	}
	if got.Status != subscription.Active || !got.PeriodEnd.Equal(through) || !got.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("subscription %s through %s, want it left as it was", got.Status, got.PeriodEnd)
	}
	if n := tierOf(t, ctx, org, "cyc-overdue"); n != tier.Free {
		t.Fatalf("tier %s while overdue, want free: the paid period and its grace are over", n)
	}

	// The customer acts: one period from now, and nothing for the months before.
	resp := invokeRenew(org, ctx, sub.Id())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("renew status=%d", resp.StatusCode)
	}
	price := lookupPlan("dev").Price
	if m.chargeCalls != 1 || m.lastChargeAmount != price {
		t.Fatalf("renew charged %d times, last %d; want once for one period (%d)", m.chargeCalls, m.lastChargeAmount, price)
	}
	got = reloadSub(t, db, sub.Id())
	if got.Status != subscription.Active || time.Since(got.PeriodStart) > time.Minute || !got.PeriodEnd.Equal(got.PeriodStart.AddDate(0, 1, 0)) {
		t.Fatalf("after renewing: %s %s..%s, want active for a month from now", got.Status, got.PeriodStart, got.PeriodEnd)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid || !invs[0].PeriodStart.Equal(got.PeriodStart) {
		t.Fatalf("invoices %+v, want the one period from now, paid", invs)
	}
	if r := cycleAt(t, ctx, org, got.PeriodStart.Add(time.Hour), false); len(acted(r)) != 0 || m.chargeCalls != 1 {
		t.Fatalf("the next run acted: %+v", acted(r))
	}
}

// TestCycle_WithinGraceRenewsOnePeriod: 71 hours late is inside the grace
// window; the renewal charges exactly one period, starting where the paid one
// ended.
func TestCycle_WithinGraceRenewsOnePeriod(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-grace")
	m := squareMock("", "", "sqpay_grace")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	through := now.Add(-71 * time.Hour)
	sub := cardSubThrough(t, db, "cyc-grace", through)

	only(t, cycleAt(t, ctx, org, now, false), engine.Renewed)
	got := reloadSub(t, db, sub.Id())
	if m.chargeCalls != 1 || m.lastChargeAmount != lookupPlan("dev").Price {
		t.Fatalf("charges=%d last=%d, want one period", m.chargeCalls, m.lastChargeAmount)
	}
	if !got.PeriodStart.Equal(through) || !got.PeriodEnd.Equal(through.AddDate(0, 1, 0)) {
		t.Fatalf("renewed onto %s..%s, want one month from %s", got.PeriodStart, got.PeriodEnd, through)
	}
}

// TestCycle_CancelAtPeriodEnd: a subscriber who canceled at period end is ended
// then, charged nothing, and drops to free.
func TestCycle_CancelAtPeriodEnd(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-cancel")
	m := squareMock("cust_c", "ccof_c", "sqpay_c")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	sub := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev"}`)
	if _, err := CancelSubscription(ctx, org, sub.Id(), true); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if tierOf(t, ctx, org, "cyc-cancel") != tier.Pro {
		t.Fatal("a cancel at period end keeps the tier until then")
	}

	only(t, cycleAt(t, ctx, org, sub.PeriodEnd, false), engine.CanceledAtPeriodEnd)
	got := reloadSub(t, db, sub.Id())
	if m.chargeCalls != 1 || len(invoicesForSub(t, db, sub.Id())) != 1 {
		t.Fatalf("charges=%d invoices=%d, want only the subscribe", m.chargeCalls, len(invoicesForSub(t, db, sub.Id())))
	}
	if got.Status != subscription.Canceled || !got.Ended.Equal(sub.PeriodEnd) {
		t.Fatalf("subscription %s ended %s, want canceled at %s", got.Status, got.Ended, sub.PeriodEnd)
	}
	if n := tierOf(t, ctx, org, "cyc-cancel"); n != tier.Free {
		t.Fatalf("tier = %s, want free", n)
	}
}

// TestCycle_DeclinedRenewalRetriesThenDropsToFree: a declined renewal keeps the
// tier and the plan while it is retried at exactly +1d, +3d and +7d, each on a
// new Square idempotency key, and the last decline ends the subscription.
func TestCycle_DeclinedRenewalRetriesThenDropsToFree(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-dunning")
	m := squareMock("", "", "")
	m.chargeErr = squareDecline()
	withFakeSquare(t, m)
	devAllotment(t, 5)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-dunning", d)

	for _, st := range []struct {
		at      time.Duration
		want    engine.Action
		charges int
	}{
		{0, engine.RenewalFailed, 1},
		{time.Hour, engine.Skipped, 1},
		{24*time.Hour - time.Second, engine.Skipped, 1},
		{24 * time.Hour, engine.RetryFailed, 2},
		{24*time.Hour + time.Minute, engine.Skipped, 2},
		{72*time.Hour - time.Second, engine.Skipped, 2},
		{72 * time.Hour, engine.RetryFailed, 3},
		{168*time.Hour - time.Second, engine.Skipped, 3},
		{168 * time.Hour, engine.Uncollectible, 4},
	} {
		r := cycleAt(t, ctx, org, d.Add(st.at), false)
		only(t, r, st.want)
		if m.chargeCalls != st.charges {
			t.Fatalf("at +%s: charges %d, want %d", st.at, m.chargeCalls, st.charges)
		}
		if r.Allotments.Granted != 0 {
			t.Fatalf("at +%s: granted %d allotments; a past_due subscriber keeps the tier and nothing else", st.at, r.Allotments.Granted)
		}
		if st.want == engine.Uncollectible {
			break
		}
		if n := tierOf(t, ctx, org, "cyc-dunning"); n != tier.Pro {
			t.Fatalf("at +%s: tier %s, want Pro kept through the retries", st.at, n)
		}
		view, err := ReadTier(ctx, org, "cyc-dunning", tier.Pro)
		if err != nil || view.Plan != servedAs("dev") {
			t.Fatalf("at +%s: served as %q (err %v), want the plan kept with the tier", st.at, view.Plan, err)
		}
	}

	keys := make([]string, 0, len(m.chargedKeys))
	for k := range m.chargedKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != 4 {
		t.Fatalf("Square saw %d distinct idempotency keys %v, want one per attempt", len(keys), keys)
	}
	for i, k := range keys {
		if !strings.HasSuffix(k, ":attempt:"+string(rune('0'+i))) || !strings.Contains(k, ":period:") {
			t.Fatalf("key %d = %q, want (subscription, period, attempt %d)", i, k, i)
		}
	}

	inv := invoicesForSub(t, db, sub.Id())[0]
	if inv.Status != billinginvoice.Uncollectible || inv.AttemptCount != 4 {
		t.Fatalf("invoice %s after %d attempts, want uncollectible after 4", inv.Status, inv.AttemptCount)
	}
	if got := reloadSub(t, db, sub.Id()); got.Status != subscription.Canceled || got.Ended.IsZero() {
		t.Fatalf("subscription %s ended %s, want canceled", got.Status, got.Ended)
	}
	if n := tierOf(t, ctx, org, "cyc-dunning"); n != tier.Free {
		t.Fatalf("tier = %s after the last retry, want free", n)
	}
}

// TestCycle_RetrySucceedsOnTheSecondAttempt: the card declines at renewal and
// goes through on the +1d retry; the invoice is paid and the subscription is
// active on the period it paid for.
func TestCycle_RetrySucceedsOnTheSecondAttempt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-recover")
	m := squareMock("", "", "sqpay_recover")
	m.chargeErr = squareDecline()
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-recover", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	m.chargeErr = nil
	res := only(t, cycleAt(t, ctx, org, d.Add(24*time.Hour), false), engine.Retried)

	if m.chargeCalls != 2 || res.AmountCents != lookupPlan("dev").Price {
		t.Fatalf("charges=%d amount=%d, want the retry to charge one period", m.chargeCalls, res.AmountCents)
	}
	inv := invoicesForSub(t, db, sub.Id())[0]
	if inv.Status != billinginvoice.Paid || inv.Id() != res.InvoiceId {
		t.Fatalf("invoice %s %s, want %s paid", inv.Id(), inv.Status, res.InvoiceId)
	}
	got := reloadSub(t, db, sub.Id())
	if got.Status != subscription.Active || !got.PeriodStart.Equal(d) || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
		t.Fatalf("subscription %s %s..%s, want active on the paid month", got.Status, got.PeriodStart, got.PeriodEnd)
	}
}

// TestCycle_CustomerPaysThePastDueInvoice: a past_due subscriber who pays the
// open renewal invoice themselves is moved onto the paid period by the next
// run, which charges nothing; the invoice stays in the invoice list.
func TestCycle_CustomerPaysThePastDueInvoice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-selfpay")
	m := squareMock("", "", "sqpay_selfpay")
	m.chargeErr = squareDecline()
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-selfpay", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	m.chargeErr = nil
	if resp := invokePay(org, ctx, inv.Id()); resp.StatusCode != http.StatusOK {
		t.Fatalf("pay status=%d", resp.StatusCode)
	}
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 1 || invs[0].Status != billinginvoice.Paid {
		t.Fatalf("after paying, the invoice list holds %d invoices; want the one, paid", len(invs))
	}

	res := only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.Renewed)
	if res.AmountCents != 0 || m.chargeCalls != 2 {
		t.Fatalf("the run charged %d (card calls %d), want nothing more", res.AmountCents, m.chargeCalls)
	}
	if got := reloadSub(t, db, sub.Id()); got.Status != subscription.Active || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
		t.Fatalf("subscription %s through %s, want active on the paid month", got.Status, got.PeriodEnd)
	}
}

// TestCycle_SeatRowsEndWithTheirTeam: a team subscription that ends takes its
// members' seat rows with it, so the members drop to free as well.
func TestCycle_SeatRowsEndWithTheirTeam(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-team")
	m := squareMock("", "", "sqpay_team")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	p, err := resolveSubscriptionPlan(db, "team")
	if err != nil {
		t.Fatalf("resolve team: %v", err)
	}
	owner, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId: "cyc-team", PlanId: "team", Quantity: minSeats("team"), Members: []string{"cyc-team/bob"},
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	owner.ProviderType = "square"
	owner.EndCancel = true
	if err := owner.Update(); err != nil {
		t.Fatalf("update team: %v", err)
	}
	if tierOf(t, ctx, org, "cyc-team/bob") != tier.Pro {
		t.Fatal("a seat on a live team confers Pro")
	}

	r := cycleAt(t, ctx, org, owner.PeriodEnd, false)
	acts := actionsOf(r)
	if acts[owner.Id()] != engine.CanceledAtPeriodEnd || len(acted(r)) != 2 {
		t.Fatalf("results %+v, want the team canceled and its seat ended", r.Results)
	}
	for _, res := range r.Results {
		if res.UserId == "cyc-team/bob" && res.Action != engine.EndedWithParent {
			t.Fatalf("seat row action %s, want ended_with_parent", res.Action)
		}
	}
	if n := tierOf(t, ctx, org, "cyc-team/bob"); n != tier.Free {
		t.Fatalf("member tier = %s after the team ended, want free", n)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charged %d times, want none", m.chargeCalls)
	}
}

// TestCycle_SeatRowsOutliveAnEscalatedTeam: a team whose renewal attempt was
// escalated (unpaid, the outcome not yet known) has not ended, so its members'
// seat rows are kept for when the payment is resolved.
func TestCycle_SeatRowsOutliveAnEscalatedTeam(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-team-esc")
	withFakeSquare(t, squareMock("", "", "sqpay_team_esc"))
	db := datastore.New(org.Namespaced(ctx))

	p, err := resolveSubscriptionPlan(db, "team")
	if err != nil {
		t.Fatalf("resolve team: %v", err)
	}
	owner, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId: "cyc-team-esc", PlanId: "team", Quantity: minSeats("team"), Members: []string{"cyc-team-esc/bob"},
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	owner.ProviderType = "square"
	owner.Status = subscription.Unpaid
	if err := owner.Update(); err != nil {
		t.Fatalf("update team: %v", err)
	}

	cycleAt(t, ctx, org, owner.PeriodEnd.Add(-time.Hour), false)
	seats, err := orgSubscriptions(db, "cyc-team-esc/bob")
	if err != nil || len(seats) != 1 {
		t.Fatalf("seat rows %d (%v), want one", len(seats), err)
	}
	if seats[0].Status == subscription.Canceled {
		t.Fatalf("seat row %s (%v), want it kept while the team's payment is unresolved", seats[0].Status, seats[0].Metadata["endReason"])
	}
}

// TestCycle_CompedPlansStayActiveAndAreNeverCharged: a paid ecosystem org's own
// account with no payment recorded at all (provisioned on enterprise terms) is
// never charged or ended. Past its period end it moves on to the period holding
// now, keeping its tier.
func TestCycle_CompedPlansStayActiveAndAreNeverCharged(t *testing.T) {
	for _, tc := range []struct{ org, subject string }{
		{"hanzo", "hanzo"},
		{"lux", "lux"},
	} {
		t.Run(tc.org, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg(tc.org)
			m := squareMock("", "", "sqpay_comp")
			withFakeSquare(t, m)
			db := datastore.New(org.Namespaced(ctx))
			now := time.Now()
			sub := unpaidSub(t, db, tc.subject, "dev", now.AddDate(0, -3, 0))

			res := only(t, cycleAt(t, ctx, org, now, false), engine.Comped)
			if res.Source != sourceComped || m.chargeCalls != 0 || len(invoicesForSub(t, db, sub.Id())) != 0 {
				t.Fatalf("paid by %q, %d charges, %d invoices; want comped with none", res.Source, m.chargeCalls, len(invoicesForSub(t, db, sub.Id())))
			}
			got := reloadSub(t, db, sub.Id())
			if got.Status != subscription.Active || now.Before(got.PeriodStart) || !now.Before(got.PeriodEnd) {
				t.Fatalf("row %s %s..%s, want active on the period holding now", got.Status, got.PeriodStart, got.PeriodEnd)
			}
			if r := cycleAt(t, ctx, org, now.Add(time.Hour), false); len(acted(r)) != 0 {
				t.Fatalf("a caught-up comped row was acted on again: %+v", acted(r))
			}
		})
	}
}

// TestCycle_VoidedRenewalInvoiceEndsTheRow: a declined renewal whose invoice
// is voided ends at the end of the period it paid for; it is not left past due,
// serving its tier.
func TestCycle_VoidedRenewalInvoiceEndsTheRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-voided")
	m := squareMock("", "", "")
	m.chargeErr = squareDecline()
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-voided", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	if _, f := VoidInvoiceIn(ctx, org, invoicesForSub(t, db, sub.Id())[0].Id(), nil); f != nil {
		t.Fatalf("void: %+v", f)
	}
	only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.EndedInvoiceVoided)
	if got := reloadSub(t, db, sub.Id()); got.Status != subscription.Canceled || !got.Ended.Equal(d) {
		t.Fatalf("row %s ended %s, want canceled at %s", got.Status, got.Ended, d)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charged %d times, want only the declined renewal", m.chargeCalls)
	}
}

// TestCycle_GiftEndsAtItsGrantedTime: a gift is never charged, and it ends when
// the time it was granted for does; a gift with no end recorded stays.
func TestCycle_GiftEndsAtItsGrantedTime(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-gift")
	m := squareMock("", "", "sqpay_gift")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	timed := unpaidSub(t, db, "cyc-gift/timed", "dev", now.Add(-time.Hour))
	open := unpaidSub(t, db, "cyc-gift/open", "dev", now)
	for _, s := range []*subscription.Subscription{timed, open} {
		s.ProviderType = gift.ProviderType
		if s == open {
			s.PeriodEnd = time.Time{}
		}
		if err := s.Update(); err != nil {
			t.Fatalf("mark gift: %v", err)
		}
	}

	res := only(t, cycleAt(t, ctx, org, now, false), engine.GiftEnded)
	if res.SubscriptionId != timed.Id() || res.Source != sourceGift || m.chargeCalls != 0 {
		t.Fatalf("result %+v, %d charges; want the timed gift ended with no charge", res, m.chargeCalls)
	}
	if got := reloadSub(t, db, timed.Id()); got.Status != subscription.Canceled || !got.Ended.Equal(timed.PeriodEnd) {
		t.Fatalf("timed gift %s ended %s, want canceled at its granted end", got.Status, got.Ended)
	}
	if n := tierOf(t, ctx, org, "cyc-gift/timed"); n != tier.Free {
		t.Fatalf("tier %s after the gift ended, want free", n)
	}
	if got := reloadSub(t, db, open.Id()); got.Status != subscription.Active {
		t.Fatalf("a gift with no end recorded is %s, want active", got.Status)
	}
}

// unpaidSub stores slug for subject as an admin opens it: provider "internal", no
// card, no invoice — nothing recorded about a payment — paid through `through`.
func unpaidSub(t *testing.T, db *datastore.Datastore, subject, slug string, through time.Time) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, slug)
	if err != nil {
		t.Fatalf("resolve %s: %v", slug, err)
	}
	s := subscription.New(db)
	s.UserId = subject
	s.Quantity = 1
	engine.StartSubscription(s, p)
	s.ProviderType = "internal"
	s.PeriodEnd = through
	s.PeriodStart = through.AddDate(0, -1, 0)
	if err := s.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	return s
}

// TestRenewalOf_HowItWasBoughtDecidesInEveryOrg: a subscription renews from the
// source it was bought from, in every org: a card-bought plan on its card, one
// bought with credits or the balance from prepaid money, one collected outside
// Hanzo by that processor. Only a gift, and a paid ecosystem org's own account
// provisioned on enterprise terms, are never charged; a plan with no payment
// recorded at all has nothing to renew it.
func TestRenewalOf_HowItWasBoughtDecidesInEveryOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	withFakeSquare(t, squareMock("", "", "sqpay_how"))
	now := time.Now()
	for _, tc := range []struct {
		org, subject, bought string
		want                 string
	}{
		{"hanzo", "hanzo/alice", "card", sourceCard},
		{"acme", "hanzo/alice", "card", sourceCard},
		{"hanzo", "hanzo/bob", "balance", sourcePrepaid},
		{"hanzo", "hanzo", "credit", sourceComped},
		{"hanzo", "hanzo/carol", "credit", sourcePrepaid},
		{"acme", "acme/carol", "credit", sourcePrepaid},
		{"hanzo", "hanzo", "", sourceComped},
		{"acme", "acme/dave", "", sourceNone},
		{"acme", "acme/erin", "gift", sourceGift},
		{"acme", "acme/fay", "external", sourceExternal},
	} {
		org := moneyOrg(tc.org)
		db := datastore.New(org.Namespaced(ctx))
		var sub *subscription.Subscription
		switch tc.bought {
		case "card":
			sub = cardSubThrough(t, db, tc.subject, now)
			if _, err := engine.CreatePaidFirstInvoice(db, sub, "card", "sqpay_first"); err != nil {
				t.Fatalf("first invoice: %v", err)
			}
		case "balance", "credit":
			sub = unpaidSub(t, db, tc.subject, "dev", now)
			if _, err := engine.CreatePaidFirstInvoice(db, sub, tc.bought, "first"); err != nil {
				t.Fatalf("first invoice: %v", err)
			}
		case "gift":
			sub = unpaidSub(t, db, tc.subject, "dev", now)
			sub.ProviderType = gift.ProviderType
		case "external":
			sub = unpaidSub(t, db, tc.subject, "dev", now)
			sub.Type = subscription.External
		default:
			sub = unpaidSub(t, db, tc.subject, "dev", now)
		}
		_, source, err := renewalOf(org, db, sub, chargeProviderForOrg(org), prepaidFor(ctx, org))
		if err != nil || source != tc.want {
			t.Errorf("%s in %s bought %q: renews from %q (err %v), want %q", tc.subject, tc.org, tc.bought, source, err, tc.want)
		}
	}
}

// TestCycle_OtherModeEnds: a subscription bought in test mode is not charged
// through a live org's processor, and it does not keep its tier forever: it ends
// at its period end.
func TestCycle_OtherModeEnds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-mode")
	m := squareMock("", "", "sqpay_mode")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	s := cardSubThrough(t, db, "cyc-mode", now.Add(-time.Hour))
	s.Test = true
	if err := s.Update(); err != nil {
		t.Fatalf("mark test: %v", err)
	}

	only(t, cycleAt(t, ctx, org, now, false), engine.EndedOtherMode)
	if m.chargeCalls != 0 {
		t.Fatalf("charged %d times, want none", m.chargeCalls)
	}
	if got := reloadSub(t, db, s.Id()); got.Status != subscription.Canceled || !got.Ended.Equal(s.PeriodEnd) {
		t.Fatalf("row %s ended %s, want canceled at its period end", got.Status, got.Ended)
	}
}

// TestCycle_DryRunReportsTheSameActions: a dry run charges nothing, writes
// nothing, and reports exactly the actions and amounts the real run then takes.
func TestCycle_DryRunReportsTheSameActions(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-dry")
	m := squareMock("", "", "sqpay_dry")
	withFakeSquare(t, m)
	devAllotment(t, 5)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()

	due := cardSubThrough(t, db, "cyc-dry/due", now.Add(-time.Hour))
	overdue := cardSubThrough(t, db, "cyc-dry/overdue", now.AddDate(0, -3, 0))
	leaving := cardSubThrough(t, db, "cyc-dry/leaving", now.Add(-time.Hour))
	leaving.EndCancel = true
	if err := leaving.Update(); err != nil {
		t.Fatalf("cancel at period end: %v", err)
	}
	ids := []string{due.Id(), overdue.Id(), leaving.Id()}
	before := make(map[string]time.Time)
	for _, id := range ids {
		before[id] = reloadSub(t, db, id).UpdatedAt
	}

	dry := cycleAt(t, ctx, org, now, true)
	want := map[string]engine.Action{
		due.Id():     engine.Renewed,
		overdue.Id(): engine.Overdue,
		leaving.Id(): engine.CanceledAtPeriodEnd,
	}
	if got := actionsOf(dry); len(got) != len(want) || got[due.Id()] != want[due.Id()] ||
		got[overdue.Id()] != want[overdue.Id()] || got[leaving.Id()] != want[leaving.Id()] {
		t.Fatalf("dry run actions %v, want %v", got, want)
	}
	if !dry.DryRun || dry.ChargedCents != lookupPlan("dev").Price || dry.Allotments.Granted != 1 {
		t.Fatalf("dry run charged %d, granted %d; want one period and dev's grant reported", dry.ChargedCents, dry.Allotments.Granted)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("dry run charged the card %d times", m.chargeCalls)
	}
	for _, id := range ids {
		if got := reloadSub(t, db, id); !got.UpdatedAt.Equal(before[id]) {
			t.Fatalf("dry run wrote subscription %s", id)
		}
		if n := len(invoicesForSub(t, db, id)); n != 0 {
			t.Fatalf("dry run stored %d invoices for %s", n, id)
		}
	}
	if grantedThisMonth(t, ctx, org, "cyc-dry/due") {
		t.Fatal("dry run granted the allotment")
	}

	real := cycleAt(t, ctx, org, now, false)
	if got := actionsOf(real); got[due.Id()] != want[due.Id()] || got[overdue.Id()] != want[overdue.Id()] ||
		got[leaving.Id()] != want[leaving.Id()] || real.ChargedCents != dry.ChargedCents || real.Allotments.Granted != dry.Allotments.Granted {
		t.Fatalf("real run %v charged %d granted %d; the dry run said %v, %d, %d",
			got, real.ChargedCents, real.Allotments.Granted, actionsOf(dry), dry.ChargedCents, dry.Allotments.Granted)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("real run charged %d times, want 1", m.chargeCalls)
	}
}

// grantedThisMonth reports whether subject holds this month's allotment grant.
func grantedThisMonth(t *testing.T, ctx context.Context, org *organization.Organization, subject string) bool {
	t.Helper()
	view, err := ReadRollup(ctx, org, subject, "", time.Now())
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}
	return view.Included.GrantedCents > 0
}

// TestCycle_UnknownChargeIsRepeatedUnderItsKey: a charge that fails in transit
// may have landed. It is not counted as a decline: the subscription keeps its
// tier, the grace window never voids the invoice, and every later run repeats the
// same attempt, which Square answers from its first try instead of charging
// again. Still unknown once the retry schedule is spent, it is escalated: the
// subscription is unpaid (no tier) and the attempt stays on the invoice for an
// operator.
func TestCycle_UnknownChargeIsRepeatedUnderItsKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-unknown")
	m := squareMock("", "", "")
	m.chargeErr = errors.New("read tcp: i/o timeout")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-unknown", d)

	for _, at := range []time.Duration{0, time.Hour, 100 * time.Hour} {
		res := only(t, cycleAt(t, ctx, org, d.Add(at), false), engine.Skipped)
		if !strings.Contains(res.Reason, "same idempotency key") {
			t.Fatalf("at +%s: reason %q, want the charge repeated under its key", at, res.Reason)
		}
		if n := tierOf(t, ctx, org, "cyc-unknown"); n != tier.Pro {
			t.Fatalf("at +%s: tier %s, want Pro kept", at, n)
		}
	}
	only(t, cycleAt(t, ctx, org, d.Add(200*time.Hour), false), engine.Escalated)
	if m.chargeCalls != 1 || len(m.chargedKeys) != 1 {
		t.Fatalf("Square saw %d charges under %d keys, want one attempt repeated under one key", m.chargeCalls, len(m.chargedKeys))
	}
	inv := invoicesForSub(t, db, sub.Id())[0]
	if inv.Status != billinginvoice.Open || inv.AttemptCount != 0 || inv.PendingKey == "" {
		t.Fatalf("invoice %s after %d counted attempts (pending %q), want open with the attempt still recorded", inv.Status, inv.AttemptCount, inv.PendingKey)
	}
	if got := reloadSub(t, db, sub.Id()); got.Status != subscription.Unpaid {
		t.Fatalf("subscription %s, want unpaid", got.Status)
	}
	if n := tierOf(t, ctx, org, "cyc-unknown"); n != tier.Free {
		t.Fatalf("tier %s once escalated, want free", n)
	}
}

// TestRunSubscriptionCycle_EveryOrg: the scheduler's entry point walks every
// organization, a dry run of it charges nothing, and the run-all endpoint runs
// the same cycle for real.
func TestRunSubscriptionCycle_EveryOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	m := squareMock("", "", "sqpay_all")
	withFakeSquare(t, m)
	now := time.Now()

	due := make(map[string]bool)
	for _, name := range []string{"cyc-all-a", "cyc-all-b"} {
		org := organization.New(datastore.New(ctx))
		org.Name = name
		org.Live = true
		if err := org.Create(); err != nil {
			t.Fatalf("create org %s: %v", name, err)
		}
		due[cardSubThrough(t, datastore.New(org.Namespaced(ctx)), name, now.Add(-time.Hour)).Id()] = true
	}

	dry, err := RunSubscriptionCycle(ctx, nil, nil, now, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Orgs < 2 || len(dry.Results) != 2 || m.chargeCalls != 0 {
		t.Fatalf("dry run over %d orgs: %+v, charges %d; want both subscriptions reported and nothing charged", dry.Orgs, dry.Results, m.chargeCalls)
	}
	for _, r := range dry.Results {
		if !due[r.SubscriptionId] || r.Action != engine.Renewed {
			t.Fatalf("dry run result %+v, want a renewal of a due subscription", r)
		}
	}

	// The endpoint with no dryRun is a dry run; only dryRun=false charges.
	runAll := func(target string) map[string]any {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		resp := driveSeeded(func(c *zip.Ctx) { c.SetContext(ctx) }, "/v1/billing/cycle/run-all", req, RunBillingCycleAllOrgs)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", target, resp.StatusCode)
		}
		return jsonBody(t, resp)
	}
	body := runAll("/v1/billing/cycle/run-all")
	if body["dryRun"] != true || body["chargedCents"] != float64(2*lookupPlan("dev").Price) || m.chargeCalls != 0 {
		t.Fatalf("run-all with no dryRun answered %v with %d charges; want a dry run reporting both", body, m.chargeCalls)
	}
	body = runAll("/v1/billing/cycle/run-all?dryRun=false")
	if body["dryRun"] != false || body["chargedCents"] != float64(2*lookupPlan("dev").Price) || m.chargeCalls != 2 {
		t.Fatalf("run-all?dryRun=false answered %v with %d charges; want both renewed", body, m.chargeCalls)
	}
}

// shiftAsOldCode moves a freshly subscribed row one period ahead of its paid
// first invoice: a row whose sale moved the period on as it recorded the charge.
func shiftAsOldCode(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, step func(time.Time) time.Time) {
	t.Helper()
	row := reloadSub(t, db, sub.Id())
	row.PeriodStart, row.PeriodEnd = row.PeriodEnd, step(row.PeriodEnd)
	if err := row.Update(); err != nil {
		t.Fatalf("shift: %v", err)
	}
}

// runCycle runs the cycle endpoint for org with the given query and answers the
// status.
func runCycle(t *testing.T, ctx context.Context, org *organization.Organization, query string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cycle/run"+query, nil)
	return driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", org)
	}, "/v1/billing/cycle/run", req, RunBillingCycle).StatusCode
}

// realignAt runs the realignment endpoint for org with the given query.
func realignAt(t *testing.T, ctx context.Context, org *organization.Organization, query string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/realign/run"+query, nil)
	resp := driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
		c.Locals("organization", org)
	}, "/v1/billing/realign/run", req, RealignSubscriptions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("realign%s status=%d", query, resp.StatusCode)
	}
	return jsonBody(t, resp)
}

// TestCycle_OldRowsAreRealignedOnlyWhenAsked: a monthly and an annual row sit
// one period ahead of what they paid for. The cycle never moves them: a dry run
// reports them pending realignment and settles nothing for them, and a live run
// is refused while they are. The realignment endpoint is a dry run unless told
// otherwise: it reports the move and writes nothing, and only dryRun=false
// moves the row back onto the paid invoice's period. From there the cycle
// renews it when that period ends, within the grace window; one realigned long
// after is overdue, and nothing is charged until the customer acts.
func TestCycle_OldRowsAreRealignedOnlyWhenAsked(t *testing.T) {
	year := int64(lookupPlan("dev").AnnualTotal)
	for _, tc := range []struct {
		name  string
		body  string
		price int64
		step  func(time.Time) time.Time
	}{
		{"monthly", `{"sourceId":"cnon:ok","planId":"dev"}`, lookupPlan("dev").Price, func(at time.Time) time.Time { return at.AddDate(0, 1, 0) }},
		{"annual", `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`, year, func(at time.Time) time.Time { return at.AddDate(1, 0, 0) }},
	} {
		for _, age := range []struct {
			name string
			at   func(paid time.Time) time.Time
			want engine.Action
		}{
			{"an hour after", func(paid time.Time) time.Time { return paid.Add(time.Hour) }, engine.Renewed},
			{"ten days after", func(paid time.Time) time.Time { return paid.Add(10 * 24 * time.Hour) }, engine.Overdue},
		} {
			t.Run(tc.name+" "+age.name, func(t *testing.T) {
				ctx := ae.NewContext()
				defer ctx.Close()
				org := moneyOrg("cyc-old-" + tc.name)
				m := squareMock("cust_o", "ccof_o", "sqpay_o")
				withFakeSquare(t, m)
				db := datastore.New(org.Namespaced(ctx))

				sub := subscribeByCard(t, ctx, org, tc.body)
				start, paid := sub.PeriodStart, sub.PeriodEnd
				shiftAsOldCode(t, db, sub, tc.step)
				at := age.at(paid)

				for _, dry := range []bool{true, false} {
					if res := only(t, cycleAt(t, ctx, org, at, dry), realignPending); res.AmountCents != 0 {
						t.Fatalf("dry=%v: the cycle charged %d for a row pending realignment", dry, res.AmountCents)
					}
				}
				if code := runCycle(t, ctx, org, "?dryRun=false"); code != http.StatusConflict || m.chargeCalls != 1 {
					t.Fatalf("a live run before the realignment answered %d after %d charges, want 409 and only the subscribe", code, m.chargeCalls)
				}
				if got := reloadSub(t, db, sub.Id()); !got.PeriodStart.Equal(paid) {
					t.Fatalf("the cycle moved the row to %s", got.PeriodStart)
				}

				dry := realignAt(t, ctx, org, "")
				if dry["dryRun"] != true || dry["realigned"] != float64(1) {
					t.Fatalf("realign with no dryRun answered %v, want a dry run finding the row", dry)
				}
				if got := reloadSub(t, db, sub.Id()); !got.PeriodStart.Equal(paid) {
					t.Fatalf("a dry-run realignment moved the row to %s", got.PeriodStart)
				}
				live := realignAt(t, ctx, org, "?dryRun=false")
				if live["dryRun"] != false || live["realigned"] != float64(1) {
					t.Fatalf("realign?dryRun=false answered %v", live)
				}
				if got := reloadSub(t, db, sub.Id()); !got.PeriodStart.Equal(start) || !got.PeriodEnd.Equal(paid) {
					t.Fatalf("realigned row %s..%s, want the paid period %s..%s", got.PeriodStart, got.PeriodEnd, start, paid)
				}
				if again := realignAt(t, ctx, org, "?dryRun=false"); again["realigned"] != float64(0) {
					t.Fatalf("a second realignment found %v", again["realigned"])
				}

				res := only(t, cycleAt(t, ctx, org, at, false), age.want)
				wantCalls := 1 // the subscribe
				if age.want == engine.Renewed {
					wantCalls = 2
					if res.AmountCents != tc.price {
						t.Fatalf("renewal charged %d, want one period at %d", res.AmountCents, tc.price)
					}
				}
				if m.chargeCalls != wantCalls {
					t.Fatalf("card charged %d times, want %d", m.chargeCalls, wantCalls)
				}
				got := reloadSub(t, db, sub.Id())
				switch age.want {
				case engine.Renewed:
					if !got.PeriodStart.Equal(paid) || !got.PeriodEnd.Equal(tc.step(paid)) || got.Status != subscription.Active {
						t.Fatalf("row %s %s..%s, want active on %s..%s", got.Status, got.PeriodStart, got.PeriodEnd, paid, tc.step(paid))
					}
				case engine.Overdue:
					if !got.PeriodEnd.Equal(paid) || got.Status != subscription.Active {
						t.Fatalf("row %s through %s, want it left active through %s", got.Status, got.PeriodEnd, paid)
					}
				}
			})
		}
	}
}

// TestCycle_UnrecordedPaymentEndsAndFreePlanRenews: a paid plan whose row and
// invoices record no way it was paid and that has no card ends at its period
// end with nothing charged; a plan with no fee renews for nothing.
func TestCycle_UnrecordedPaymentEndsAndFreePlanRenews(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-internal")
	m := squareMock("", "", "sqpay_int")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()

	open := func(subject, slug string) *subscription.Subscription {
		p, err := resolveSubscriptionPlan(db, slug)
		if err != nil {
			t.Fatalf("resolve %s: %v", slug, err)
		}
		s := subscription.New(db)
		s.UserId = subject
		s.Quantity = 1
		engine.StartSubscription(s, p)
		s.ProviderType = "internal"
		s.PeriodEnd = now.Add(-time.Hour)
		s.PeriodStart = s.PeriodEnd.AddDate(0, -1, 0)
		if err := s.Create(); err != nil {
			t.Fatalf("create: %v", err)
		}
		return s
	}
	paid := open("cyc-internal/paid", "dev")
	free := open("cyc-internal/free", "free")

	r := cycleAt(t, ctx, org, now, false)
	byID := make(map[string]CycleResult)
	for _, res := range r.Results {
		byID[res.SubscriptionId] = res
	}
	if res := byID[paid.Id()]; res.Action != engine.EndedNoCard || res.Source != sourceNone {
		t.Fatalf("unrecorded paid plan: %+v, want ended_no_card", res)
	}
	if res := byID[free.Id()]; res.Action != engine.Renewed || res.AmountCents != 0 {
		t.Fatalf("free plan: %+v, want renewed for nothing", res)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("card charged %d times, want none", m.chargeCalls)
	}
	if got := reloadSub(t, db, paid.Id()); got.Status != subscription.Canceled || !got.Ended.Equal(paid.PeriodEnd) {
		t.Fatalf("unrecorded paid plan %s ended %s, want canceled at its period end", got.Status, got.Ended)
	}
}

// TestCycle_ExpiryEmitsCanceledWithItsReason: every end the cycle makes is a
// subscription_canceled event carrying the action as its reason, so a
// notification can hang off an end.
func TestCycle_ExpiryEmitsCanceledWithItsReason(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	got := make(chan map[string]any, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e map[string]any
		_ = json.NewDecoder(r.Body).Decode(&e)
		got <- e
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	org := moneyOrg("cyc-event")
	withFakeSquare(t, squareMock("", "", "sqpay_ev"))
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-event", now.Add(-time.Hour))
	sub.EndCancel = true
	if err := sub.Update(); err != nil {
		t.Fatalf("cancel at period end: %v", err)
	}

	report := newCycleReport(now, false)
	cycleOrg(ctx, org, db, now, false, events.NewClient(srv.URL), chargeProviderForOrg(org), "", report)
	only(t, report, engine.CanceledAtPeriodEnd)
	select {
	case e := <-got:
		props, _ := e["properties"].(map[string]any)
		if e["event"] != events.EventSubscriptionCanceled || props["reason"] != string(engine.CanceledAtPeriodEnd) || props["subscription_id"] != sub.Id() {
			t.Fatalf("event %v, want subscription_canceled for %s with reason canceled_at_period_end", e, sub.Id())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event reached the collector")
	}
}

// TestCycle_ChargeAfterCancelEmitsOnlyThePayment: a renewal that lands after an
// immediate cancel reports charged_after_cancel with the amount to refund, keeps
// the row canceled, and emits invoice_paid alone — the cancel emitted its own
// subscription_canceled.
func TestCycle_ChargeAfterCancelEmitsOnlyThePayment(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	got := make(chan map[string]any, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e map[string]any
		_ = json.NewDecoder(r.Body).Decode(&e)
		got <- e
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	org := moneyOrg("cyc-cancel-charge")
	withFakeSquare(t, squareMock("", "", "sqpay_cc"))
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-cancel-charge", now.Add(-time.Hour))
	card := chargeProviderForOrg(org)
	charger := func(c context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amount int64) (string, error) {
		if _, err := cancelSubscription(ctx, org, sub.Id(), false); err != nil {
			t.Errorf("cancel: %v", err)
		}
		return card(c, db, inv, amount)
	}

	report := newCycleReport(now, false)
	cycleOrg(ctx, org, db, now, false, events.NewClient(srv.URL), charger, "", report)
	res := only(t, report, engine.ChargedAfterCancel)
	if res.AmountCents == 0 || report.ChargedCents != res.AmountCents {
		t.Fatalf("result %+v charged %d, want the collected amount reported", res, report.ChargedCents)
	}
	if row := reloadSub(t, db, sub.Id()); row.Status != subscription.Canceled {
		t.Fatalf("row %s, want canceled", row.Status)
	}
	select {
	case e := <-got:
		if e["event"] != events.EventInvoicePaid {
			t.Fatalf("event %v, want invoice_paid", e["event"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event reached the collector")
	}
	select {
	case e := <-got:
		t.Fatalf("second event %v, want invoice_paid alone", e["event"])
	case <-time.After(300 * time.Millisecond):
	}
}

// stallSquare is a processor whose charges do not answer while stall is set
// until the caller's context ends, or settle after five seconds; otherwise it
// is the mock.
type stallSquare struct {
	*mockSquareProcessor
	stall bool
	keys  []string
}

func (s *stallSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	s.keys = append(s.keys, req.IdempotencyKey)
	if s.stall {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return s.mockSquareProcessor.Charge(ctx, req)
}

// TestCycle_AChargeThatDoesNotAnswerIsUnknown: a processor that has not answered
// within chargeTimeout leaves the attempt recorded and the row past due, and the
// next run repeats it under the same key.
func TestCycle_AChargeThatDoesNotAnswerIsUnknown(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-stall")
	sq := &stallSquare{mockSquareProcessor: squareMock("", "", "sqpay_stall"), stall: true}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sq)
		return reg
	}
	oldTimeout := chargeTimeout
	chargeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { processorsForOrg, chargeTimeout = old, oldTimeout })
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Add(-time.Hour)
	sub := cardSubThrough(t, db, "cyc-stall", d)

	only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.Skipped)
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].PendingKey == "" || reloadSub(t, db, sub.Id()).Status != subscription.PastDue {
		t.Fatalf("invoices %+v, want one open invoice keeping its attempt on a past-due row", invs)
	}
	sq.stall = false
	only(t, cycleAt(t, ctx, org, d.Add(2*time.Hour), false), engine.Retried)
	if len(sq.keys) != 2 || sq.keys[0] != sq.keys[1] {
		t.Fatalf("keys %v, want the unanswered charge repeated under its key", sq.keys)
	}
}

// TestCycle_OrgBudgetLeavesTheRestForTheNextRun: an org whose cycle has run past
// orgCycleBudget starts no more subscriptions; the run reports the stop and the
// next run settles the rest.
func TestCycle_OrgBudgetLeavesTheRestForTheNextRun(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-budget")
	withFakeSquare(t, squareMock("", "", "sqpay_budget"))
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	cardSubThrough(t, db, "cyc-budget-a", now.Add(-time.Hour))
	cardSubThrough(t, db, "cyc-budget-b", now.Add(-time.Hour))
	card := chargeProviderForOrg(org)
	slow := func(c context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amount int64) (string, error) {
		time.Sleep(30 * time.Millisecond)
		return card(c, db, inv, amount)
	}
	old := orgCycleBudget
	orgCycleBudget = 10 * time.Millisecond
	t.Cleanup(func() { orgCycleBudget = old })

	report := newCycleReport(now, false)
	cycleOrg(ctx, org, db, now, false, nil, slow, "", report)
	if len(report.Results) != 1 || report.Results[0].Action != engine.Renewed || len(report.Errors) != 1 {
		t.Fatalf("results %+v errors %v, want one renewal and the stop reported", report.Results, report.Errors)
	}
	orgCycleBudget = old
	next := newCycleReport(now, false)
	cycleOrg(ctx, org, db, now, false, nil, slow, "", next)
	renewed := 0
	for _, r := range next.Results {
		if r.Action == engine.Renewed {
			renewed++
		}
	}
	if renewed != 1 || len(next.Errors) != 0 {
		t.Fatalf("next run %+v errors %v, want the other subscription renewed", next.Results, next.Errors)
	}
}

// invoiceRow reads the invoice with id, bound to db.
func invoiceRow(db *datastore.Datastore, id string) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	if err := inv.GetById(id); err != nil {
		return nil, err
	}
	return inv, nil
}

// acted is the report's results with the subscriptions it took a step on: every
// line but the ones not yet due.
func acted(r *CycleReport) []CycleResult {
	out := make([]CycleResult, 0, len(r.Results))
	for _, res := range r.Results {
		if res.Action != notDue {
			out = append(out, res)
		}
	}
	return out
}

// reprice sets what slug sells at, monthly and for a year, for the life of one
// test: on the plan authority row an admin edits, and on the embedded catalog.
func reprice(t *testing.T, ctx context.Context, slug string, month, year int64) {
	t.Helper()
	p := lookupPlan(slug)
	if p == nil {
		t.Fatalf("plan %q missing from the catalog", slug)
	}
	wasMonth, wasYear := p.Price, p.AnnualTotal
	p.Price, p.AnnualTotal = month, year
	t.Cleanup(func() { p.Price, p.AnnualTotal = wasMonth, wasYear })
	row := plan.New(plan.AuthorityDB(ctx))
	if err := row.GetById(slug); err != nil {
		t.Fatalf("read the authority row of %q: %v", slug, err)
	}
	row.Price, row.AnnualTotal, row.PriceAnnual = currency.Cents(month), currency.Cents(year), currency.Cents(year/12)
	if err := row.Update(); err != nil {
		t.Fatalf("reprice %q: %v", slug, err)
	}
	if again, err := resolveSubscriptionPlan(datastore.New(ctx), slug); err != nil || int64(again.Price) != month {
		t.Fatalf("the plan now resolves at %v (err %v), want %d", again, err, month)
	}
}

// TestCycle_RenewsAtTheIntervalAndPriceItWasBought: a subscription renews on the
// interval it was bought on, at the price it was bought at. The catalog raising
// the plan's price afterwards changes nothing for it: a monthly one is charged
// its old monthly price each month, a yearly one its old annual total each year.
func TestCycle_RenewsAtTheIntervalAndPriceItWasBought(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		step       func(time.Time) time.Time
	}{
		{"monthly", `{"sourceId":"cnon:ok","planId":"dev"}`, func(at time.Time) time.Time { return at.AddDate(0, 1, 0) }},
		{"yearly", `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`, func(at time.Time) time.Time { return at.AddDate(1, 0, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("cyc-bought-" + tc.name)
			m := squareMock("cust_b", "ccof_b", "sqpay_b")
			withFakeSquare(t, m)
			db := datastore.New(org.Namespaced(ctx))

			sub := subscribeByCard(t, ctx, org, tc.body)
			bought := m.lastChargeAmount
			if !sub.PeriodEnd.Equal(tc.step(sub.PeriodStart)) {
				t.Fatalf("bought %s..%s, want one %s", sub.PeriodStart, sub.PeriodEnd, tc.name)
			}
			dev := lookupPlan("dev")
			reprice(t, ctx, "dev", dev.Price*3, dev.AnnualTotal*3)

			end := sub.PeriodEnd
			for renewal := 1; renewal <= 2; renewal++ {
				res := only(t, cycleAt(t, ctx, org, end, false), engine.Renewed)
				if m.lastChargeAmount != bought || res.AmountCents != bought || res.PriceCents != bought {
					t.Fatalf("renewal %d charged %d (reported %d, price %d), want the %d it was bought at", renewal, m.lastChargeAmount, res.AmountCents, res.PriceCents, bought)
				}
				got := reloadSub(t, db, sub.Id())
				if !got.PeriodStart.Equal(end) || !got.PeriodEnd.Equal(tc.step(end)) {
					t.Fatalf("renewal %d onto %s..%s, want one %s from %s", renewal, got.PeriodStart, got.PeriodEnd, tc.name, end)
				}
				end = got.PeriodEnd
			}
		})
	}
}

// TestCycle_CreditBoughtRenewsFromCredits: a plan bought with credits renews
// from the subscriber's prepaid money — its credit grants, then its balance —
// and never from a card, even one saved to its account.
func TestCycle_CreditBoughtRenewsFromCredits(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("cyc-credit")
	m := squareMock("cust_cr", "ccof_cr", "sqpay_cr")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	price := lookupPlan("dev").Price
	g := grant(t, db, "cyc-credit", 2*price)
	seedSavedCard(t, db, "cyc-credit", "ccof_cr", "cust_cr")

	sale, err := Subscribe(ctx, org, SubscribeIn{SourceID: "credits", PlanID: "dev", Subject: "cyc-credit"})
	if err != nil {
		t.Fatalf("subscribe with credits: %v", err)
	}
	sub := reloadSub(t, db, sale.SubscriptionID)
	res := only(t, cycleAt(t, ctx, org, sub.PeriodEnd, false), engine.Renewed)
	if res.Source != sourcePrepaid || res.AmountCents != price || m.chargeCalls != 0 {
		t.Fatalf("renewal %+v with %d card charges, want %d from prepaid money and no card", res, m.chargeCalls, price)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != 0 {
		t.Fatalf("credit grant holds %d (err %v), want both periods drawn from it", left.RemainingCents, err)
	}
	if got := reloadSub(t, db, sub.Id()); !got.PeriodStart.Equal(sub.PeriodEnd) || got.Status != subscription.Active {
		t.Fatalf("row %s from %s, want active on the next period", got.Status, got.PeriodStart)
	}
}

// TestCycle_DryRunReportsEachSubscription: the dry run's report names, for every
// live subscription, its org, plan, the interval it was bought on, the period it
// has paid for and the one the action bills, what it costs, the source that
// renews it and the action the live run would take — and writes nothing.
func TestCycle_DryRunReportsEachSubscription(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-report")
	m := squareMock("", "", "sqpay_rep")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now().Truncate(time.Second)
	price := lookupPlan("dev").Price

	due := cardSubThrough(t, db, "cyc-report/due", now.Add(-time.Hour))
	overdue := cardSubThrough(t, db, "cyc-report/overdue", now.AddDate(0, -2, 0))
	current := cardSubThrough(t, db, "cyc-report/current", now.AddDate(0, 0, 10))
	before := reloadSub(t, db, due.Id()).UpdatedAt

	r := cycleAt(t, ctx, org, now, true)
	if !r.DryRun || len(r.Results) != 3 {
		t.Fatalf("report dry=%v with %d results, want a dry run listing all three", r.DryRun, len(r.Results))
	}
	byID := make(map[string]CycleResult)
	for _, res := range r.Results {
		byID[res.SubscriptionId] = res
		if res.Org != org.Name || res.Plan != "dev" || res.Interval != "month" || res.IntervalCount != 1 ||
			res.Source != sourceCard || res.PriceCents != price || res.Currency != "usd" {
			t.Fatalf("result %+v, want org, plan, interval, source and price named", res)
		}
	}
	d := byID[due.Id()]
	if d.Action != engine.Renewed || d.AmountCents != price || !d.PeriodEnd.Equal(due.PeriodEnd) ||
		!d.BillStart.Equal(due.PeriodEnd) || !d.BillEnd.Equal(due.PeriodEnd.AddDate(0, 1, 0)) || d.InvoiceId != "" {
		t.Fatalf("due: %+v, want renewed for %d over the month after %s", d, price, due.PeriodEnd)
	}
	if o := byID[overdue.Id()]; o.Action != engine.Overdue || o.AmountCents != 0 || !o.PeriodEnd.Equal(overdue.PeriodEnd) {
		t.Fatalf("overdue: %+v, want overdue with nothing charged", o)
	}
	if c := byID[current.Id()]; c.Action != notDue || c.AmountCents != 0 || !c.PeriodEnd.Equal(current.PeriodEnd) {
		t.Fatalf("current: %+v, want not_due", c)
	}
	if r.ChargedCents != price || m.chargeCalls != 0 {
		t.Fatalf("report charged %d with %d card calls, want %d reported and no card touched", r.ChargedCents, m.chargeCalls, price)
	}
	if got := reloadSub(t, db, due.Id()); !got.UpdatedAt.Equal(before) || len(invoicesForSub(t, db, due.Id())) != 0 {
		t.Fatal("the dry run wrote the subscription or stored an invoice")
	}
}

// TestCycle_DryRunDeclinesACardRenewalWithNoCard: a card-bought subscription
// whose card is gone is reported by the dry run as the declined renewal the live
// run makes, never as renewed.
func TestCycle_DryRunDeclinesACardRenewalWithNoCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-nocard")
	m := squareMock("", "", "sqpay_nocard")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-nocard", now.Add(-time.Hour))
	if _, err := engine.CreatePaidFirstInvoice(db, sub, "card", "sqpay_first"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	row := reloadSub(t, db, sub.Id())
	row.DefaultPaymentMethod = ""
	if err := row.Update(); err != nil {
		t.Fatalf("remove card: %v", err)
	}

	dry := only(t, cycleAt(t, ctx, org, now, true), engine.RenewalFailed)
	live := only(t, cycleAt(t, ctx, org, now, false), engine.RenewalFailed)
	if dry.Source != sourceCard || dry.AmountCents != 0 || live.AmountCents != 0 || m.chargeCalls != 0 {
		t.Fatalf("dry %+v live %+v with %d charges; want a declined card renewal reported the same, nothing charged", dry, live, m.chargeCalls)
	}
}
