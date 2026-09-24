package billing

import (
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
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
