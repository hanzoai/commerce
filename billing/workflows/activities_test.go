package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// These prove the money flows the Temporal workflows (subscription.go +
// dunning.go) orchestrate, at the seam where money actually moves: the
// BillingActivities methods, driven against the real SQLite datastore + the
// real billing/engine. The activities read the org db via orgDB(ctx, org),
// whose queries resolve the namespace from the CONTEXT, so every test seeds its
// data and calls the activity with the SAME namespaced context.

// wctx returns a base test context plus a namespaced context in org `ns` — the
// exact shape the activity's orgDB(ctx, ns) reads from.
func wctx(t *testing.T, ns string) (ae.Context, context.Context) {
	t.Helper()
	base := ae.NewContext()
	t.Cleanup(base.Close)
	return base, nscontext.WithNamespace(base, ns)
}

func seedDB(ctx context.Context) *datastore.Datastore {
	return datastore.New(ctx)
}

// flatPlan is a $price/month single-seat plan.
func flatPlan(name string, price int64) plan.Plan {
	return plan.Plan{
		Name:          name,
		Price:         currency.Cents(price),
		Currency:      currency.USD,
		Interval:      types.Monthly,
		IntervalCount: 1,
	}
}

// fundBalance credits userId's spendable balance the way deductFromBalance reads
// it: a Deposit to (kind "user", id userId) in the given currency.
func fundBalance(t *testing.T, db *datastore.Datastore, userId string, cents int64) {
	t.Helper()
	tx := transaction.New(db)
	tx.Type = transaction.Deposit
	tx.DestinationKind = "user"
	tx.DestinationId = userId
	tx.Currency = currency.USD
	tx.Amount = currency.Cents(cents)
	if err := tx.Create(); err != nil {
		t.Fatalf("fund balance: %v", err)
	}
}

func openInvoice(t *testing.T, db *datastore.Datastore, userId string, cents int64) *billinginvoice.BillingInvoice {
	t.Helper()
	inv := billinginvoice.New(db)
	inv.UserId = userId
	inv.Currency = currency.USD
	inv.Subtotal = cents
	inv.SetNumber(1)
	if err := inv.Finalize(); err != nil { // draft -> open, AmountDue = Subtotal
		t.Fatalf("finalize invoice: %v", err)
	}
	if err := inv.Create(); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return inv
}

// TestTransitionSubscriptionActivity proves the status-write the workflows use to
// move a subscription active<->unpaid (dunning's terminal transition, and the
// trial-end/collection-success transition).
func TestTransitionSubscriptionActivity(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	sub := subscription.New(db)
	sub.UserId = "acme/alice"
	sub.Status = subscription.Active
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	a := &BillingActivities{}
	if err := a.TransitionSubscriptionActivity(ctx, TransitionParams{
		OrgName: ns, SubscriptionId: sub.Id(), NewStatus: "unpaid",
	}); err != nil {
		t.Fatalf("transition: %v", err)
	}

	reloaded := subscription.New(db)
	if err := reloaded.GetById(sub.Id()); err != nil {
		t.Fatalf("reload sub: %v", err)
	}
	if reloaded.Status != subscription.Unpaid {
		t.Fatalf("status = %s, want unpaid", reloaded.Status)
	}
}

func TestTransitionSubscriptionActivity_NotFound(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	a := &BillingActivities{}
	err := a.TransitionSubscriptionActivity(ctx, TransitionParams{
		OrgName: ns, SubscriptionId: "does-not-exist", NewStatus: "active",
	})
	if err == nil {
		t.Fatal("want error for a missing subscription, got nil")
	}
}

// TestCancelSubscriptionActivity proves both cancel modes the lifecycle workflow
// signals: immediate (status canceled) and at-period-end (stays active, flagged).
func TestCancelSubscriptionActivity(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)
	a := &BillingActivities{}

	t.Run("immediate", func(t *testing.T) {
		sub := subscription.New(db)
		sub.UserId = "acme/now"
		sub.Status = subscription.Active
		if err := sub.Create(); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := a.CancelSubscriptionActivity(ctx, CancelParams{
			OrgName: ns, SubscriptionId: sub.Id(), AtPeriodEnd: false,
		}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		got := subscription.New(db)
		if err := got.GetById(sub.Id()); err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.Status != subscription.Canceled || !got.Canceled {
			t.Fatalf("immediate cancel = status:%s canceled:%v, want canceled/true", got.Status, got.Canceled)
		}
	})

	t.Run("at_period_end", func(t *testing.T) {
		sub := subscription.New(db)
		sub.UserId = "acme/later"
		sub.Status = subscription.Active
		if err := sub.Create(); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := a.CancelSubscriptionActivity(ctx, CancelParams{
			OrgName: ns, SubscriptionId: sub.Id(), AtPeriodEnd: true,
		}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		got := subscription.New(db)
		if err := got.GetById(sub.Id()); err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.Status != subscription.Active || !got.EndCancel {
			t.Fatalf("period-end cancel = status:%s endCancel:%v, want active/true", got.Status, got.EndCancel)
		}
	})
}

// TestChangePlanActivity proves the plan-change signal path swaps the persisted
// plan pointer.
func TestChangePlanActivity(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	pro := plan.New(db)
	pro.Name = "Pro"
	pro.Price = currency.Cents(2000)
	pro.Currency = currency.USD
	pro.Interval = types.Monthly
	pro.IntervalCount = 1
	if err := pro.Create(); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = "acme/dee"
	sub.Status = subscription.Active
	sub.Plan = flatPlan("Basic", 1000)
	sub.PlanId = "plan_basic"
	sub.Quantity = 1
	sub.PeriodStart = now.Add(-24 * time.Hour)
	sub.PeriodEnd = now.Add(24 * time.Hour)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	a := &BillingActivities{}
	if err := a.ChangePlanActivity(ctx, ChangePlanParams{
		OrgName: ns, SubscriptionId: sub.Id(), NewPlanId: pro.Id(), Prorate: false,
	}); err != nil {
		t.Fatalf("change plan: %v", err)
	}

	got := subscription.New(db)
	if err := got.GetById(sub.Id()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.PlanId != pro.Id() {
		t.Fatalf("planId = %q, want %q", got.PlanId, pro.Id())
	}
}

// TestRenewSubscriptionActivity_Succeeds proves the renewal lifecycle: a due
// subscription generates an invoice, collects it (a $0 plan collects trivially),
// advances the period, and reports success — the happy path of the recurring
// billing loop in SubscriptionLifecycleWorkflow.
func TestRenewSubscriptionActivity_Succeeds(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = "acme/free"
	sub.Status = subscription.Active
	sub.Plan = flatPlan("Free", 0)
	sub.PlanId = "plan_free"
	sub.Quantity = 1
	sub.PeriodStart = now.AddDate(0, -2, 0)
	sub.PeriodEnd = now.AddDate(0, -1, 0) // elapsed -> due
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	priorEnd := sub.PeriodEnd

	a := &BillingActivities{}
	res, err := a.RenewSubscriptionActivity(ctx, RenewalParams{OrgName: ns, SubscriptionId: sub.Id()})
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !res.Success {
		t.Fatalf("renew result = %+v, want success", res)
	}
	if res.InvoiceId == "" {
		t.Fatal("renew produced no invoice id")
	}
	if !res.NextPeriodEnd.After(priorEnd) {
		t.Fatalf("period did not advance: next=%v prior=%v", res.NextPeriodEnd, priorEnd)
	}
}

// TestRenewSubscriptionActivity_PaidFromBalance proves a priced renewal collects
// from the user's prepaid balance and marks the invoice paid.
func TestRenewSubscriptionActivity_PaidFromBalance(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	const user = "acme/payer"
	fundBalance(t, db, user, 5000) // $50 covers the $20 charge

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = user
	sub.Status = subscription.Active
	sub.Plan = flatPlan("Pro", 2000)
	sub.PlanId = "plan_pro"
	sub.Quantity = 1
	sub.PeriodStart = now.AddDate(0, -2, 0)
	sub.PeriodEnd = now.AddDate(0, -1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	a := &BillingActivities{}
	res, err := a.RenewSubscriptionActivity(ctx, RenewalParams{OrgName: ns, SubscriptionId: sub.Id()})
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !res.Success {
		t.Fatalf("renew from balance = %+v, want success", res)
	}

	inv := billinginvoice.New(db)
	if err := inv.GetById(res.InvoiceId); err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	if inv.Status != billinginvoice.Paid {
		t.Fatalf("invoice status = %s, want paid", inv.Status)
	}
}

// TestRenewSubscriptionActivity_FailsThenDunningExhausts proves the failed-payment
// path the dunning workflow drives end-to-end: renewal with no funds leaves the
// invoice open and the subscription PastDue; the dunning terminal activities then
// mark the invoice uncollectible and transition the subscription to unpaid.
func TestRenewSubscriptionActivity_FailsThenDunningExhausts(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = "acme/broke"
	sub.Status = subscription.Active
	sub.Plan = flatPlan("Pro", 2000)
	sub.PlanId = "plan_pro"
	sub.Quantity = 1
	sub.PeriodStart = now.AddDate(0, -2, 0)
	sub.PeriodEnd = now.AddDate(0, -1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	a := &BillingActivities{}
	res, err := a.RenewSubscriptionActivity(ctx, RenewalParams{OrgName: ns, SubscriptionId: sub.Id()})
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if res.Success {
		t.Fatalf("renew with no funds = %+v, want failure", res)
	}

	// Subscription went PastDue (dunning is now entered by the workflow).
	past := subscription.New(db)
	if err := past.GetById(sub.Id()); err != nil {
		t.Fatalf("reload sub: %v", err)
	}
	if past.Status != subscription.PastDue {
		t.Fatalf("status after failed renew = %s, want past_due", past.Status)
	}

	// Dunning exhausts -> MarkUncollectibleActivity + TransitionSubscriptionActivity(unpaid).
	if err := a.MarkUncollectibleActivity(ctx, MarkUncollectibleParams{
		OrgName: ns, InvoiceId: res.InvoiceId,
	}); err != nil {
		t.Fatalf("mark uncollectible: %v", err)
	}
	if err := a.TransitionSubscriptionActivity(ctx, TransitionParams{
		OrgName: ns, SubscriptionId: sub.Id(), NewStatus: "unpaid",
	}); err != nil {
		t.Fatalf("transition unpaid: %v", err)
	}

	inv := billinginvoice.New(db)
	if err := inv.GetById(res.InvoiceId); err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	if inv.Status != billinginvoice.Uncollectible {
		t.Fatalf("invoice status = %s, want uncollectible", inv.Status)
	}
	final := subscription.New(db)
	if err := final.GetById(sub.Id()); err != nil {
		t.Fatalf("reload sub: %v", err)
	}
	if final.Status != subscription.Unpaid {
		t.Fatalf("final status = %s, want unpaid", final.Status)
	}
}

// TestCollectInvoiceActivity proves the dunning retry's core: a re-collection
// attempt on an open invoice succeeds when the balance covers it and reports
// failure (invoice left open) when it does not.
func TestCollectInvoiceActivity(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)
	a := &BillingActivities{}

	t.Run("succeeds_from_balance", func(t *testing.T) {
		const user = "acme/collect-ok"
		fundBalance(t, db, user, 3000)
		inv := openInvoice(t, db, user, 2000)

		res, err := a.CollectInvoiceActivity(ctx, CollectInvoiceParams{OrgName: ns, InvoiceId: inv.Id()})
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if !res.Success {
			t.Fatal("collect with sufficient balance = failure, want success")
		}
		got := billinginvoice.New(db)
		if err := got.GetById(inv.Id()); err != nil {
			t.Fatalf("reload invoice: %v", err)
		}
		if got.Status != billinginvoice.Paid {
			t.Fatalf("invoice status = %s, want paid", got.Status)
		}
	})

	t.Run("fails_no_funds", func(t *testing.T) {
		const user = "acme/collect-broke"
		inv := openInvoice(t, db, user, 2000)

		res, err := a.CollectInvoiceActivity(ctx, CollectInvoiceParams{OrgName: ns, InvoiceId: inv.Id()})
		if err != nil {
			t.Fatalf("collect: %v", err)
		}
		if res.Success {
			t.Fatal("collect with no funds = success, want failure")
		}
		got := billinginvoice.New(db)
		if err := got.GetById(inv.Id()); err != nil {
			t.Fatalf("reload invoice: %v", err)
		}
		if got.Status != billinginvoice.Open {
			t.Fatalf("invoice status = %s, want open (unpaid, awaiting retry)", got.Status)
		}
	})
}

// TestMarkUncollectibleActivity proves the dunning terminal write in isolation.
func TestMarkUncollectibleActivity(t *testing.T) {
	const ns = "acme"
	_, ctx := wctx(t, ns)
	db := seedDB(ctx)

	inv := openInvoice(t, db, "acme/give-up", 2000)

	a := &BillingActivities{}
	if err := a.MarkUncollectibleActivity(ctx, MarkUncollectibleParams{OrgName: ns, InvoiceId: inv.Id()}); err != nil {
		t.Fatalf("mark uncollectible: %v", err)
	}
	got := billinginvoice.New(db)
	if err := got.GetById(inv.Id()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != billinginvoice.Uncollectible {
		t.Fatalf("status = %s, want uncollectible", got.Status)
	}
}
