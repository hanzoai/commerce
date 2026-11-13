package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A trial opened before commerce stopped selling trials runs to its end and ends
// there: it is never due, card or no card, it confers nothing once over, and the
// renewal at its end charges nothing and ends it.
func TestAnOpenTrialEndsAtItsEndUnbilled(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("trial-end")

	p := makePlan("plan_trial_end", "Pro", 2000, types.Monthly, 1)
	now := time.Now()
	sub := subscription.New(db)
	sub.UserId, sub.DefaultPaymentMethod, sub.ProviderType = "trial-end/ann", "pm_ann", "square"
	sub.Plan, sub.PlanId, sub.Status = *p, p.Id(), subscription.Trialing
	sub.TrialStart, sub.TrialEnd = now.AddDate(0, 0, -7), now.Add(time.Hour)
	sub.PeriodStart, sub.PeriodEnd = sub.TrialEnd, sub.TrialEnd.AddDate(0, 1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if Over(sub, now) || IsDue(sub, now) {
		t.Fatalf("an open trial: over=%v due=%v, want neither", Over(sub, now), IsDue(sub, now))
	}
	after := sub.TrialEnd.Add(time.Hour)
	if !Over(sub, after) || IsDue(sub, after) {
		t.Fatalf("a trial past its end: over=%v due=%v, want over and not due", Over(sub, after), IsDue(sub, after))
	}
	pre := &card{}
	inv, res, err := renewAt(context.Background(), db, sub, nil, pre.charge, after)
	if err != nil || inv != nil || res == nil || res.Success {
		t.Fatalf("the trial's end: inv=%v res=%+v err=%v, want it ended with nothing invoiced", inv, res, err)
	}
	if len(pre.charges) != 0 {
		t.Fatalf("the trial's end charged the card %v", pre.charges)
	}
	if sub.Status != subscription.Canceled || !sub.Ended.Equal(sub.TrialEnd) {
		t.Fatalf("after its end the trial is %s, ended %v; want canceled at %v", sub.Status, sub.Ended, sub.TrialEnd)
	}
}
