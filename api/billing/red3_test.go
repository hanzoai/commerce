package billing

import (
	"context"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/allotment"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/zap-proto/zip"
)

// slowSquare answers each charge after a second, as Square does under load.
type slowSquare struct{ *MockSquareProcessor }

func (s slowSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	time.Sleep(1100 * time.Millisecond)
	return s.MockSquareProcessor.Charge(ctx, req)
}

// Two renew requests for one overdue row that both read the row before either
// holds the org's cycle lock (a double-click, or a client retry while the org's
// cycle is running) each carry the row to their own "now" and charge a period.
func TestRed3_DoubleRenewOfOverdueChargesTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-dbl")
	m := slowSquare{squareMock("", "", "sqpay_dbl")}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(m)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	db := datastore.New(org.Namespaced(ctx))
	through := time.Now().AddDate(0, -3, 0)
	sub := cardSubThrough(t, db, "red3-dbl", through)

	unlock := lockOrgCycle(org.Name) // the org's cycle is running
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) { defer wg.Done(); codes[i] = renewSlow(org, ctx, sub.Id()) }(i)
	}
	time.Sleep(300 * time.Millisecond)
	unlock()
	wg.Wait()

	got := reloadSub(t, db, sub.Id())
	paid := 0
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		t.Logf("invoice %s %s %s..%s paid=%d", inv.Id(), inv.Status, inv.PeriodStart.Format(time.RFC3339), inv.PeriodEnd.Format(time.RFC3339), inv.AmountPaid)
		if inv.Status == billinginvoice.Paid {
			paid++
		}
	}
	t.Logf("codes=%v charges=%d row %s %s..%s", codes, m.chargeCalls, got.Status, got.PeriodStart.Format(time.RFC3339), got.PeriodEnd.Format(time.RFC3339))
	if m.chargeCalls != 1 || paid != 1 {
		t.Fatalf("one overdue row renewed twice: %d card charges, %d paid invoices; want one", m.chargeCalls, paid)
	}
}

// An overdue row the cycle never charges confers nothing once its paid period
// and the grace after it are over: the tier is free. Its customer acts by
// subscribing again, which is allowed, and the new subscription replaces the
// overdue one without billing the months between.
func TestRed3_OverdueRowKeepsPaidTierForever(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-tier")
	m := squareMock("", "", "sqpay_tier")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	through := now.AddDate(0, -3, 0)
	cardSubThrough(t, db, "red3-tier", through)
	for _, at := range []time.Time{now, now.AddDate(0, 6, 0)} {
		only(t, cycleAt(t, ctx, org, at, false), engine.Overdue)
	}
	tr := tierOf(t, ctx, org, "red3-tier")
	t.Logf("tier after 3 unpaid months (and 6 more cycle months): %s, charges=%d", tr, m.chargeCalls)
	if tr != "free" {
		t.Fatalf("an overdue row, paid through %s and never charged again, serves tier %q", through.Format(time.RFC3339), tr)
	}

	old := subsOf(t, db, "red3-tier")[0]
	fresh := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","userId":"red3-tier"}`)
	if fresh.Id() == old.Id() || fresh.Status != subscription.Active || m.chargeCalls != 1 || m.lastChargeAmount != lookupPlan("dev").Price {
		t.Fatalf("subscribing again: row %s %s after %d charges of %d; want a new active row and one period charged",
			fresh.Id(), fresh.Status, m.chargeCalls, m.lastChargeAmount)
	}
	if got := reloadSub(t, db, old.Id()); got.Status != subscription.Canceled || !got.Ended.Equal(old.PeriodEnd) || got.Metadata["endReason"] != string(engine.Replaced) {
		t.Fatalf("the overdue row is %s ended %s (%v); want it replaced at its paid period's end", got.Status, got.Ended, got.Metadata["endReason"])
	}
	if n := tierOf(t, ctx, org, "red3-tier"); n != "pro" {
		t.Fatalf("tier %s after subscribing again, want pro", n)
	}
	for _, inv := range invoicesForSub(t, db, old.Id()) {
		if inv.Status == billinginvoice.Paid && inv.PeriodStart.After(through.Add(-time.Second)) {
			t.Fatalf("the months after %s were billed on invoice %s", through, inv.Id())
		}
	}
}

// A past_due subscriber (tier kept during retries) is not "already paying", so
// buying the plan again opens a second paid row beside the one being retried.
func TestRed3_PastDueSubscriberCanBuyASecondPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-pd")
	db := datastore.New(org.Namespaced(ctx))
	s := cardSubThrough(t, db, "red3-pd", time.Now().Add(-time.Hour))
	s.Status = subscription.PastDue
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	if held := billingSubscription(db, "red3-pd", org.TestMode()); held == nil {
		t.Fatalf("a past_due subscriber (keeps its tier, retried on days 1, 3, 7) is not held: a second purchase is allowed and both renew")
	}
}

// renewSlow is invokeRenew without the 1s test deadline.
func renewSlow(org *organization.Organization, ctx context.Context, subID string) int {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscriptions/"+subID+"/renew", nil)
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(zip.MethodAll, "/v1/billing/subscriptions/:id/renew", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(ctx)
		return c.Next()
	}, RenewBillingSubscription)
	resp, err := app.Test(req, zip.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	if err != nil {
		panic(err)
	}
	return resp.StatusCode
}

// A seat on a paying annual team is granted its month only in the seat row's
// first month: nothing moves a seat row's period, and the grant now skips a
// row whose period is over.
func TestRed3_SeatMemberLosesAllotmentAfterMonthOne(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-seat")
	withFakeSquare(t, squareMock("", "", "sqpay_seat"))
	db := datastore.New(org.Namespaced(ctx))
	p := lookupPlan("team")
	if p == nil {
		t.Fatal("no team plan")
	}
	was := p.Limits
	lims := plan.Limits{}
	if was != nil {
		lims = *was
	}
	usd := 10
	lims.IncludedCloudCredits = &usd
	p.Limits = &lims
	t.Cleanup(func() { p.Limits = was })
	if IncludedMonthlyCents("team") <= 0 {
		t.Fatal("fixture: team has no allotment")
	}

	tp, err := resolveSubscriptionPlan(db, "team")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := createSubscription(db, tp, &createSubscriptionRequest{
		UserId: "red3-seat", PlanId: "team", Quantity: minSeats("team"), Members: []string{"red3-seat/bob"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The team is paid a year ahead.
	owner.ProviderType = "square"
	owner.PeriodEnd = owner.PeriodStart.AddDate(1, 0, 0)
	if err := owner.Update(); err != nil {
		t.Fatal(err)
	}
	month2 := time.Now().AddDate(0, 1, 5)
	r := cycleAt(t, ctx, org, month2, false)
	t.Logf("allotments at month 2: %+v", r.Allotments)
	seats, _ := orgSubscriptions(db, "red3-seat/bob")
	for _, s := range seats {
		t.Logf("seat row %s %s..%s", s.Status, s.PeriodStart.Format(time.RFC3339), s.PeriodEnd.Format(time.RFC3339))
	}
	if !grantedThisMonthAt(t, db, "red3-seat/bob", month2) {
		t.Fatalf("bob holds a seat on a team paid through %s and was not granted month 2", owner.PeriodEnd.Format(time.RFC3339))
	}
}

func grantedThisMonthAt(t *testing.T, db *datastore.Datastore, subject string, at time.Time) bool {
	t.Helper()
	return allotment.GrantedCents(db, subject, at, false) > 0 || allotment.GrantedCents(db, subject, at, true) > 0
}

// A charge Square took and answered 502 for leaves the renewal invoice pending.
// The customer then cancels immediately. No run asks for the money again: the
// attempt is looked up, Square cannot say it never landed (a completed payment
// cannot be canceled by its key), so every run reports it for an operator and
// the invoice keeps the attempt until one reconciles it.
func TestRed3_ImmediateCancelOrphansAPendingCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-orphan")
	d := &darkSquare{MockSquareProcessor: newMockSquare(nil, "", nil), dark: true, landed: map[string]int64{}}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(d)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "red3-orphan", now.Add(-time.Hour))
	only(t, cycleAt(t, ctx, org, now, false), engine.Skipped) // 502: unknown, pending

	if _, err := cancelSubscription(ctx, org, sub.Id(), false); err != nil {
		t.Fatal(err)
	}
	d.dark = false
	charges := len(d.landed)
	for _, at := range []time.Time{now.Add(time.Hour), now.AddDate(0, 0, 8), now.AddDate(0, 1, 0)} {
		res := only(t, cycleAt(t, ctx, org, at, false), engine.Skipped)
		if !strings.Contains(res.Reason, "operator reconciles it") {
			t.Fatalf("at %s the attempt was reported as %q, want it reported for an operator", at, res.Reason)
		}
	}
	if len(d.landed) != charges {
		t.Fatalf("a run asked Square for money again after the cancel (%d keys, want %d)", len(d.landed), charges)
	}
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		if inv.Status != billinginvoice.Open || inv.PendingKey == "" {
			t.Fatalf("invoice %s is %s with attempt %q; want the attempt kept until an operator reconciles it", inv.Id(), inv.Status, inv.PendingKey)
		}
	}
}

// A row whose renewal declined and whose retries never ran through the billed
// period (the cycle was off) is overdue. The customer renews: the attempted
// invoice is followed first, found expired, voided, and the subscription ends.
func TestRed3_CustomerRenewOfOverdueDeclinedRowEndsIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-asked")
	m := squareMock("", "", "sqpay_asked")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	through := time.Now().AddDate(0, -2, 0)
	sub := cardSubThrough(t, db, "red3-asked", through)
	sub.Status = subscription.PastDue
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	next := subscription.New(db)
	_ = next.GetById(sub.Id())
	next.PeriodStart, next.PeriodEnd = through, through.AddDate(0, 1, 0)
	inv := seedOpenInvoice(t, db, next, 2000)
	inv.AttemptCount, inv.LastAttemptAt, inv.DueDate = 1, through, through
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}

	code := renewSlow(org, ctx, sub.Id())
	got := reloadSub(t, db, sub.Id())
	t.Logf("renew=%d charges=%d row=%s endReason=%v", code, m.chargeCalls, got.Status, got.Metadata["endReason"])
	if got.Status == subscription.Canceled {
		t.Fatalf("the customer asked to renew and the subscription was ended (%v); want one period billed from now", got.Metadata["endReason"])
	}
}

func stripe(subID string, at time.Time) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(subID + "|" + strconv.FormatInt(at.Unix(), 10)))
	return h.Sum32() % 256
}

// carry takes the period lock of the new period while settle still holds the
// old one. When both keys fall on one of the 256 stripes, the goroutine locks a
// sync.Mutex it already holds and never returns.
func TestRed3_CarrySelfDeadlocksOnAStripeCollision(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-dead")
	withFakeSquare(t, squareMock("", "", "sqpay_dead"))
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red3-dead", now.AddDate(0, -2, 0))
	target := stripe(sub.Id(), now)
	through := now.AddDate(0, -2, 0)
	for stripe(sub.Id(), through) != target {
		through = through.Add(-time.Second)
	}
	sub.PeriodEnd, sub.PeriodStart = through, through.AddDate(0, -1, 0)
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	t.Logf("through=%s now=%s stripe=%d", through.Format(time.RFC3339), now.Format(time.RFC3339), target)
	done := make(chan CycleResult, 1)
	go func() {
		s := reloadSub(t, db, sub.Id())
		done <- settleOne(context.WithoutCancel(ctx), org, db, s, engine.Run{Now: now, Asked: true, Prepaid: prepaidFor(ctx, org)}, nil, railOf(org))
	}()
	select {
	case r := <-done:
		t.Logf("settled: %+v", r)
	case <-time.After(3 * time.Second):
		t.Fatalf("a customer renew of an overdue row whose old and new period keys share a stripe never returns; RenewBillingSubscription holds the org's cycle lock across it")
	}
}
