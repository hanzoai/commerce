package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// scheduled stores a Pro row bought by card that starts at start, as a sale
// following a plan paid outside checkout opens it.
func scheduled(t *testing.T, db *datastore.Datastore, user string, start time.Time) *subscription.Subscription {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId, sub.PlanId, sub.Plan = user, "dev", pro
	sub.Status = subscription.Scheduled
	sub.DefaultPaymentMethod = "pm_" + user
	sub.Start, sub.PeriodStart, sub.PeriodEnd = start, start, start
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	return sub
}

// A scheduled card plan is charged nothing before its start, is due at it, and the
// renewal that charges its first period makes it active on the period beginning
// at that start: nothing is billed for the time the plan it follows covered.
func TestScheduled_TheCardIsChargedAtTheStartAndNotBefore(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("sched-start")

	start := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	sub := scheduled(t, db, "sched-start/ada", start)
	pay := &card{}
	if IsDue(sub, start.Add(-time.Minute)) {
		t.Fatal("a scheduled row is due before its start")
	}
	if inv, _, _ := renewAt(context.Background(), db, sub, nil, pay.charge, start.Add(-time.Minute)); inv != nil || len(pay.charges) != 0 {
		t.Fatalf("billed %v and charged %v before the start", inv != nil, pay.charges)
	}
	if !IsDue(sub, start.Add(time.Minute)) {
		t.Fatal("a scheduled row on a card is not due at its start")
	}
	inv, res, err := renewAt(context.Background(), db, sub, nil, pay.charge, start.Add(time.Minute))
	if err != nil || !res.Success {
		t.Fatalf("first charge: %+v, %v", res, err)
	}
	if len(pay.charges) != 1 || pay.charges[0] != 2000 || inv.PeriodStart.Unix() != start.Unix() {
		t.Fatalf("charged %v for a period from %s; want 2000 once, from the start %s", pay.charges, inv.PeriodStart, start)
	}
	if sub.Status != subscription.Active || sub.PeriodStart.Unix() != start.Unix() || sub.PeriodEnd.Unix() != Advance(start, &sub.Plan).Unix() {
		t.Fatalf("row %s on %s..%s, want active on the period from the start", sub.Status, sub.PeriodStart, sub.PeriodEnd)
	}
}

// Declined at its start, a scheduled plan goes past due as any renewal does, and
// nothing is served.
func TestScheduled_ADeclineAtTheStartIsPastDue(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("sched-decline")

	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	sub := scheduled(t, db, "sched-decline/bo", start)
	inv, res, err := RenewSubscription(context.Background(), db, sub, nil, (&card{decline: true}).charge)
	if err != nil || res.Success || inv == nil || inv.Status != billinginvoice.Open {
		t.Fatalf("declined start: inv %v res %+v err %v; want an open invoice", inv, res, err)
	}
	if sub.Status != subscription.PastDue {
		t.Fatalf("status = %s, want past_due", sub.Status)
	}
}

// A scheduled plan canceled before its start ends at it, charged nothing.
func TestScheduled_CanceledBeforeItsStartIsNeverCharged(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("sched-cancel")

	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	sub := scheduled(t, db, "sched-cancel/cy", start)
	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	pay := &card{}
	inv, _, err := RenewSubscription(context.Background(), db, sub, nil, pay.charge)
	if err != nil || inv != nil || len(pay.charges) != 0 || sub.Status != subscription.Canceled {
		t.Fatalf("inv %v charges %v status %s err %v; want it ended, uncharged", inv, pay.charges, sub.Status, err)
	}
}

// A plan collected outside commerce never renews: once its period is over it is
// Over, and Lapse ends it as of that end. Before then it stands.
func TestExternal_LapsesAtTheEndOfItsPeriod(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("ext-lapse")

	end := time.Now().Add(time.Hour).Truncate(time.Second)
	sub := subscription.New(db)
	sub.UserId, sub.PlanId, sub.Plan = "ext-lapse/di", "dev", pro
	sub.Type, sub.Status, sub.ProviderType = subscription.External, subscription.Active, "wire"
	sub.PeriodStart, sub.PeriodEnd = end.AddDate(0, -1, 0), end
	if err := sub.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if Over(sub, end.Add(-time.Minute)) || IsDue(sub, end.Add(time.Minute)) {
		t.Fatal("an external plan is over before its end, or due after it")
	}
	if ok, err := Lapse(db, sub, end.Add(-time.Minute)); ok || err != nil {
		t.Fatalf("lapsed before its end: %v %v", ok, err)
	}
	if !Over(sub, end.Add(time.Minute)) {
		t.Fatal("an external plan past its end is not over")
	}
	ok, err := Lapse(db, sub, end.Add(time.Minute))
	if !ok || err != nil || sub.Status != subscription.Canceled || !sub.Ended.Equal(end) {
		t.Fatalf("lapse: %v %v; row %s ended %s, want canceled as of %s", ok, err, sub.Status, sub.Ended, end)
	}
}
