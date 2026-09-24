package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// lapsedCardPlan stores a card-bought dev plan for subject whose paid period
// ended three months ago and was never renewed.
func lapsedCardPlan(t *testing.T, db *datastore.Datastore, subject string) *subscription.Subscription {
	t.Helper()
	sub := cardSubThrough(t, db, subject, time.Now().AddDate(0, -3, 0))
	if _, err := engine.CreatePaidFirstInvoice(db, sub, "card", "sqpay_first"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	if err := sub.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	return reloadSub(t, db, sub.Id())
}

// TestSubscribeAgain_ReplacesALapsedPlan: on every path a customer opens a plan
// by — a card, credits, a balance record, an outside payment recorded, an
// admin's create — a subject whose paid plan lapsed is not refused as already
// paying, and the new subscription replaces the lapsed one: ended at its paid
// period's end, nothing billed for the months between.
func TestSubscribeAgain_ReplacesALapsedPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(t *testing.T, ctx ae.Context, org *organization.Organization, db *datastore.Datastore, subject string)
	}{
		{"card", func(t *testing.T, ctx ae.Context, org *organization.Organization, _ *datastore.Datastore, subject string) {
			subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","userId":"`+subject+`"}`)
		}},
		{"credits", func(t *testing.T, ctx ae.Context, org *organization.Organization, db *datastore.Datastore, subject string) {
			grant(t, db, subject, lookupPlan("dev").Price)
			if _, err := Subscribe(ctx, org, SubscribeIn{SourceID: "credits", PlanID: "dev", Subject: subject}); err != nil {
				t.Fatalf("subscribe with credits: %v", err)
			}
		}},
		{"balance record", func(t *testing.T, ctx ae.Context, org *organization.Organization, db *datastore.Datastore, subject string) {
			if _, _, err := SeedPlans(ctx); err != nil {
				t.Fatalf("seed: %v", err)
			}
			authorityPlan(t, ctx, balSlug, balSlug, "private", balPrice)
			deposit(t, db, subject, balPrice)
			in := balanceIn()
			in.Subject = subject
			if _, err := RecordSubscription(ctx, org, in); err != nil {
				t.Fatalf("record from the balance: %v", err)
			}
		}},
		{"outside payment", func(t *testing.T, ctx ae.Context, org *organization.Organization, db *datastore.Datastore, subject string) {
			in := recordDev(t, db)
			in.Subject = subject
			if got, err := RecordSubscription(ctx, org, in); err != nil || got.Outcome != RecordCreated {
				t.Fatalf("record an outside payment: %+v, %v", got, err)
			}
		}},
		{"admin create", func(t *testing.T, ctx ae.Context, org *organization.Organization, _ *datastore.Datastore, subject string) {
			if w := invokeSub(org, ctx, c1OrgAdmin, CreateBillingSubscription, `{"userId":"`+subject+`","planId":"free"}`); w.StatusCode != http.StatusCreated {
				t.Fatalf("admin create: %d %s", w.StatusCode, bodyOf(w))
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("lapse-again")
			withFakeSquare(t, squareMock("cust_la", "ccof_la", "sqpay_la"))
			db := datastore.New(org.Namespaced(ctx))
			old := lapsedCardPlan(t, db, "lapse-again")
			if held := billingSubscription(db, "lapse-again", org.TestMode()); held != nil {
				t.Fatalf("a lapsed plan is held as paying: %s", held.Id())
			}

			tc.open(t, ctx, org, db, "lapse-again")

			got := reloadSub(t, db, old.Id())
			if got.Status != subscription.Canceled || !got.Ended.Equal(old.PeriodEnd) || got.Metadata["endReason"] != string(engine.Replaced) {
				t.Fatalf("lapsed plan %s ended %s (%v), want replaced at its paid period's end %s", got.Status, got.Ended, got.Metadata["endReason"], old.PeriodEnd)
			}
			if n := len(invoicesForSub(t, db, old.Id())); n != 1 {
				t.Fatalf("the lapsed plan carries %d invoices, want only the one it was bought with", n)
			}
		})
	}
}

// TestRecordSubscription_ALatePaymentExtendsItsLapsedPlan: an outside payment
// recorded for the period after one that lapsed extends that same external plan;
// it opens no second one.
func TestRecordSubscription_ALatePaymentExtendsItsLapsedPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-late")
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	start := time.Now().AddDate(0, -2, 0).UTC().Truncate(time.Second)
	in.PeriodStart, in.PeriodEnd = start, start.AddDate(0, 1, 0)
	first, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	next := in
	next.PeriodStart, next.PeriodEnd = in.PeriodEnd, in.PeriodEnd.AddDate(0, 1, 0)
	next.Reference = map[string]string{"invoice": "inv:late"}
	got, err := RecordSubscription(ctx, org, next)
	if err != nil || got.Outcome != RecordExtended || got.Subscription.ID != first.Subscription.ID {
		t.Fatalf("late payment: %+v, %v; want the lapsed plan extended", got, err)
	}
}

// TestSubscribeAgain_GivesBackARenewalPaidAfterTheLapse: a past-due plan whose
// renewal was paid after it lapsed, while no cycle ran to move it on, is
// replaced when its customer subscribes again. Subscribing gives nothing back
// and leaves the paid invoice as it is; the next cycle gives back what it
// collected, once, to the customer's balance for a refund review.
func TestSubscribeAgain_GivesBackARenewalPaidAfterTheLapse(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("lapse-paid")
	withFakeSquare(t, squareMock("cust_lp", "ccof_lp", "sqpay_lp"))
	db := datastore.New(org.Namespaced(ctx))
	old := lapsedCardPlan(t, db, "lapse-paid")
	next := reloadSub(t, db, old.Id())
	next.PeriodStart, next.PeriodEnd = old.PeriodEnd, old.PeriodEnd.AddDate(0, 1, 0)
	late, err := engine.CreatePaidFirstInvoice(db, next, "card", "sqpay_late")
	if err != nil {
		t.Fatalf("late invoice: %v", err)
	}
	row := reloadSub(t, db, old.Id())
	row.Status = subscription.PastDue
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	before := balanceOf(t, ctx, org, "lapse-paid")
	invoice := func() *billinginvoice.BillingInvoice {
		for _, inv := range invoicesForSub(t, db, old.Id()) {
			if inv.Id() == late.Id() {
				return inv
			}
		}
		t.Fatalf("invoice %s not found", late.Id())
		return nil
	}

	subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","userId":"lapse-paid"}`)
	if got := reloadSub(t, db, old.Id()); got.Status != subscription.Canceled || got.Metadata["endReason"] != string(engine.Replaced) {
		t.Fatalf("lapsed plan %s (%v), want replaced", got.Status, got.Metadata["endReason"])
	}
	if inv := invoice(); inv.Status != billinginvoice.Paid || inv.Metadata["returnedCents"] != nil || balanceOf(t, ctx, org, "lapse-paid") != before {
		t.Fatalf("subscribing touched the paid invoice (%s, returned %v)", inv.Status, inv.Metadata["returnedCents"])
	}

	if a := actionsOf(cycleAt(t, ctx, org, time.Now(), false))[old.Id()]; a != engine.Returned {
		t.Fatalf("the cycle took %q on the replaced plan, want returned", a)
	}
	if got := balanceOf(t, ctx, org, "lapse-paid") - before; int64(got) != late.AmountPaid {
		t.Fatalf("the balance moved %d, want the %d cents the late renewal collected", got, late.AmountPaid)
	}
	if a := actionsOf(cycleAt(t, ctx, org, time.Now(), false))[old.Id()]; a != "" {
		t.Fatalf("a second cycle took %q, want nothing", a)
	}
}

// TestSubscribeAgain_EndsTheReplacedTeamsSeats: replacing a lapsed team ends
// the seats it paid for with it, and each end reaches the collector as
// subscription_canceled: the team's as replaced, the seat's as ended with its
// parent.
func TestSubscribeAgain_EndsTheReplacedTeamsSeats(t *testing.T) {
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
	org := moneyOrg("lapse-team")
	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, "team")
	if err != nil {
		t.Fatal(err)
	}
	req := func() *createSubscriptionRequest {
		return &createSubscriptionRequest{UserId: "lapse-team", PlanId: "team", Quantity: minSeats("team"), Members: []string{"lapse-team/bob"}}
	}
	old, err := createSubscription(db, p, req())
	if err != nil {
		t.Fatal(err)
	}
	old.ProviderType = "square"
	old.PeriodStart, old.PeriodEnd = time.Now().AddDate(0, -2, 0), time.Now().AddDate(0, -1, 0)
	if err := old.Update(); err != nil {
		t.Fatal(err)
	}
	fresh, err := createSubscription(db, p, req())
	if err != nil {
		t.Fatal(err)
	}

	retireLapsed(ctx, org, db, fresh, events.NewClient(srv.URL))
	seats, _ := orgSubscriptions(db, "lapse-team/bob")
	live := 0
	for _, s := range seats {
		switch {
		case bundleParentOf(s) == old.Id() && s.Status != subscription.Canceled:
			t.Fatalf("seat %s of the replaced team is %s", s.Id(), s.Status)
		case bundleParentOf(s) == fresh.Id() && s.Status == subscription.Active:
			live++
		}
	}
	if live != 1 {
		t.Fatalf("bob holds %d live seats under the new team, want 1", live)
	}
	reasons := make(map[string]string)
	for len(reasons) < 2 {
		select {
		case e := <-got:
			props, _ := e["properties"].(map[string]any)
			if e["event"] == events.EventSubscriptionCanceled {
				reasons[props["subscription_id"].(string)] = props["reason"].(string)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("events %v, want the team's and the seat's ends", reasons)
		}
	}
	if reasons[old.Id()] != string(engine.Replaced) {
		t.Fatalf("ends %v, want the team replaced", reasons)
	}
}

// TestRecordSubscription_APaymentAfterACardRebuyIsRecorded: an outside payment
// for a plan that lapsed and was replaced by a card purchase arrives late. It is
// recorded, once, for a refund review; it is not refused and moves no plan.
func TestRecordSubscription_APaymentAfterACardRebuyIsRecorded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-rebuy")
	withFakeSquare(t, squareMock("cust_rr", "ccof_rr", "sqpay_rr"))
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	in.Subject = "rec-rebuy"
	start := time.Now().AddDate(0, -3, 0).UTC().Truncate(time.Second)
	in.PeriodStart, in.PeriodEnd = start, start.AddDate(0, 1, 0)
	first, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	card := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","userId":"rec-rebuy"}`)

	late := in
	late.PeriodStart, late.PeriodEnd = in.PeriodEnd, in.PeriodEnd.AddDate(0, 1, 0)
	late.Reference = map[string]string{"invoice": "inv:late"}
	for i := 0; i < 2; i++ {
		got, err := RecordSubscription(ctx, org, late)
		if err != nil || got.Outcome != RecordUnapplied || got.Subscription.ID != first.Subscription.ID {
			t.Fatalf("late payment: %+v, %v; want it recorded against the replaced plan", got, err)
		}
	}
	evs := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(db).Filter("Type=", "payment.unapplied").GetAll(&evs); err != nil || len(evs) != 1 {
		t.Fatalf("%d payment.unapplied records (err %v), want one", len(evs), err)
	}
	if got := reloadSub(t, db, card.Id()); !got.PeriodEnd.Equal(card.PeriodEnd) || got.Status != subscription.Active {
		t.Fatalf("the card plan moved to %s %s", got.Status, got.PeriodEnd)
	}
}

// TestSubscribe_AnUnpaidPlanStillPays: a plan whose renewal attempt has no known
// outcome (unpaid) is still being paid for, so a second paid plan is refused
// beside it; once it lapses past the retry schedule the customer may buy again.
func TestSubscribe_AnUnpaidPlanStillPays(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("unpaid-pays")
	withFakeSquare(t, squareMock("cust_up", "ccof_up", "sqpay_up"))
	db := datastore.New(org.Namespaced(ctx))
	sub := cardSubThrough(t, db, "unpaid-pays", time.Now().Add(-time.Hour))
	row := reloadSub(t, db, sub.Id())
	row.Status = subscription.Unpaid
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	if held := billingSubscription(db, "unpaid-pays", org.TestMode()); held == nil || held.Id() != sub.Id() {
		t.Fatal("an unpaid plan is not held as paying")
	}
	if resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev","userId":"unpaid-pays"}`, nil); resp.StatusCode == http.StatusCreated {
		t.Fatal("a second paid plan was sold beside an unpaid one")
	}
	row = reloadSub(t, db, sub.Id())
	row.PeriodEnd = time.Now().Add(-engine.RenewalGrace - engine.RetrySchedule[len(engine.RetrySchedule)-1] - time.Hour)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	if held := billingSubscription(db, "unpaid-pays", org.TestMode()); held != nil {
		t.Fatal("an unpaid plan past the retry schedule is still held as paying")
	}
}
