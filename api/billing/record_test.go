package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan paid outside checkout: an invoice raised by hand in the processor's
// dashboard. These pin what recording it does, and what it must never do.

var (
	recStart = time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	recEnd   = time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
)

func recordDev(t *testing.T, db *datastore.Datastore) RecordIn {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, "dev")
	if err != nil || p.Price <= 0 {
		t.Fatalf("the catalog must sell a paid dev plan: %v (price %d)", err, p.Price)
	}
	return RecordIn{
		Subject:     "acme",
		PlanID:      "dev",
		PriceCents:  int64(p.Price),
		PeriodStart: recStart,
		PeriodEnd:   recEnd,
		Processor:   "square",
		Reference:   map[string]string{"invoice": "inv:0-test", "payment": "pay_test"},
		Terms:       "refundable against referred business",
	}
}

func subsOf(t *testing.T, db *datastore.Datastore, subject string) []*subscription.Subscription {
	t.Helper()
	out := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", subject).GetAll(&out); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	return out
}

func TestRecordSubscription_OpensTheExternalPlanAndMovesNoMoney(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-open")
	m := squareMock("cust_1", "ccof_1", "sqpay_1")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)

	before, err := bucketedSplit(org.Namespaced(ctx), in.Subject, currency.USD, org.TestMode())
	if err != nil {
		t.Fatalf("balance before: %v", err)
	}
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Outcome != RecordCreated {
		t.Fatalf("outcome=%q, want %q", got.Outcome, RecordCreated)
	}

	subs := subsOf(t, db, in.Subject)
	if len(subs) != 1 {
		t.Fatalf("subscriptions=%d, want exactly the one recorded", len(subs))
	}
	s := subs[0]
	if s.Status != subscription.Active || s.Type != subscription.External {
		t.Fatalf("status=%q type=%q, want active external", s.Status, s.Type)
	}
	if s.ProviderType != "square" || s.ProviderId != "" || s.DefaultPaymentMethod != "" {
		t.Fatalf("provider=%q id=%q method=%q: an external row names its processor and nothing a charge or a webhook could key on",
			s.ProviderType, s.ProviderId, s.DefaultPaymentMethod)
	}
	if !s.PeriodStart.Equal(recStart) || !s.PeriodEnd.Equal(recEnd) || !s.TrialEnd.IsZero() {
		t.Fatalf("period %s..%s trialEnd %s, want the paid period and no trial", s.PeriodStart, s.PeriodEnd, s.TrialEnd)
	}
	if int64(s.Plan.Price) != in.PriceCents || s.Plan.Slug != "dev" {
		t.Fatalf("plan %q at %d, want dev at %d", s.Plan.Slug, s.Plan.Price, in.PriceCents)
	}
	ref, _ := s.Metadata["reference"].(map[string]interface{})
	if ref["invoice"] != "inv:0-test" || s.Metadata["terms"] != in.Terms || s.Metadata["collection"] != "external" {
		t.Fatalf("metadata did not survive the store: %#v", s.Metadata)
	}
	if tier, _ := deriveTier(db, in.Subject, org.TestMode()); tier != "pro" {
		t.Fatalf("tier=%q, want the paid tier the plan confers", tier)
	}
	if held := billingSubscription(db, in.Subject, org.TestMode()); held == nil || held.Id() != s.Id() {
		t.Fatal("the recorded plan must count as the subject's one paid subscription")
	}

	if m.chargeCalls != 0 {
		t.Fatalf("processor charged %d time(s); recording a payment collected elsewhere charges nothing", m.chargeCalls)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", in.Subject).GetAll(&invs); err != nil || len(invs) != 0 {
		t.Fatalf("invoices=%d (err %v); there is nothing here to bill", len(invs), err)
	}
	after, err := bucketedSplit(org.Namespaced(ctx), in.Subject, currency.USD, org.TestMode())
	if err != nil {
		t.Fatalf("balance after: %v", err)
	}
	if after.Available != before.Available || after.CreditsRemaining != before.CreditsRemaining {
		t.Fatalf("wallet moved %+v -> %+v; recording a plan grants no credit", before, after)
	}
}

// TestRecordedPlanIsNeverChargedOrRenewedByTheProcessor: a plan collected
// outside Hanzo renews as it was bought — its processor collects the next period
// and recording that payment moves the row on. The cycle charges nothing for it,
// from prepaid money or from a card, even one on file, and neither moves nor
// ends the row.
func TestRecordedPlanIsNeverChargedOrRenewedByTheProcessor(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-never")
	m := squareMock("", "", "sqpay_never")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	if _, err := RecordSubscription(ctx, org, recordDev(t, db)); err != nil {
		t.Fatalf("record: %v", err)
	}
	s := reloadSub(t, db, subsOf(t, db, "acme")[0].Id())
	s.DefaultPaymentMethod = seedSavedCard(t, db, "acme", "ccof_never", "cust_never").Id()
	if err := s.Update(); err != nil {
		t.Fatalf("attach card: %v", err)
	}
	seedCredit(db, "acme", 50000, "topup", time.Time{})

	afterEnd := recEnd.Add(24 * time.Hour)
	if engine.IsDue(s, afterEnd) {
		t.Fatal("an externally collected plan answered due; the cycle would invoice and collect it")
	}
	for _, dry := range []bool{true, false} {
		res := only(t, cycleAt(t, ctx, org, afterEnd, dry), renewedOutside)
		if res.Source != sourceExternal || res.AmountCents != 0 {
			t.Fatalf("dry=%v: %+v, want it left to its processor with nothing charged", dry, res)
		}
	}
	burn := &countingPrepaid{}
	step, err := engine.Settle(ctx, db, reloadSub(t, db, s.Id()), engine.Run{Now: afterEnd, Prepaid: burn}, engine.PrepaidPayer(burn))
	if err != nil || step.Action != "" || burn.calls != 0 {
		t.Fatalf("settle: %+v (err %v), %d prepaid calls; it must do nothing", step, err, burn.calls)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", "acme").GetAll(&invs); err != nil || len(invs) != 0 {
		t.Fatalf("invoices=%d (err %v); there is nothing here to bill", len(invs), err)
	}
	if got := reloadSub(t, db, s.Id()); !got.PeriodEnd.Equal(recEnd) || got.Status != subscription.Active || m.chargeCalls != 0 {
		t.Fatalf("row %s through %s after %d charges; only a recorded payment moves it", got.Status, got.PeriodEnd, m.chargeCalls)
	}
}

func TestRecordSubscription_RetryIsUnchangedAndTheNextPaymentExtends(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-next")
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	in.Processor = "wire" // a processor with no card rail still holds the one paid row
	if _, err := RecordSubscription(ctx, org, in); err != nil {
		t.Fatalf("record: %v", err)
	}
	again, err := RecordSubscription(ctx, org, in)
	if err != nil || again.Outcome != RecordUnchanged {
		t.Fatalf("retry: outcome=%v err=%v, want unchanged", again, err)
	}

	next := in
	next.PeriodStart, next.PeriodEnd = recEnd, recEnd.AddDate(0, 1, 0)
	next.Reference = map[string]string{"invoice": "inv:0-next"}
	got, err := RecordSubscription(ctx, org, next)
	if err != nil || got.Outcome != RecordExtended {
		t.Fatalf("next period: outcome=%v err=%v, want extended", got, err)
	}
	subs := subsOf(t, db, in.Subject)
	if len(subs) != 1 {
		t.Fatalf("subscriptions=%d; the next payment extends the one row", len(subs))
	}
	s := subs[0]
	if !s.PeriodStart.Equal(next.PeriodStart) || !s.PeriodEnd.Equal(next.PeriodEnd) {
		t.Fatalf("period %s..%s, want the newly paid one", s.PeriodStart, s.PeriodEnd)
	}
	if periods, _ := s.Metadata["periods"].([]interface{}); len(periods) != 2 {
		t.Fatalf("periods recorded=%d, want both payments on the row", len(periods))
	}

	past := in
	past.PeriodStart, past.PeriodEnd = recStart.AddDate(0, -1, 0), recStart
	if _, err := RecordSubscription(ctx, org, past); !IsSaleRefused(err) {
		t.Fatalf("an earlier period: err=%v, want refused", err)
	}
}

func TestRecordSubscription_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("rec-price")
	db := datastore.New(org.Namespaced(ctx))
	wrong := recordDev(t, db)
	wrong.PriceCents++
	if _, err := RecordSubscription(ctx, org, wrong); !IsSaleRefused(err) {
		t.Fatalf("a payment that is not the plan's price: err=%v, want refused", err)
	}
	if n := len(subsOf(t, db, wrong.Subject)); n != 0 {
		t.Fatalf("a refused record wrote %d row(s)", n)
	}

	unknown := recordDev(t, db)
	unknown.PlanID = "no-such-plan"
	if _, err := RecordSubscription(ctx, org, unknown); !IsSaleNotFound(err) {
		t.Fatalf("an unknown plan: err=%v, want not found", err)
	}

	noRef := recordDev(t, db)
	noRef.Reference = map[string]string{"invoice": " "}
	if _, err := RecordSubscription(ctx, org, noRef); !IsSaleRefused(err) {
		t.Fatalf("no reference: err=%v, want refused", err)
	}

	paying := moneyOrg("rec-paying")
	pdb := datastore.New(paying.Namespaced(ctx))
	liveSub(t, pdb, "acme", "dev", "square")
	if _, err := RecordSubscription(ctx, paying, recordDev(t, pdb)); !IsSaleConflict(err) {
		t.Fatalf("a subject already paying through checkout: err=%v, want conflict", err)
	}
}

// TestListSubscriptions_ReadsTheRowsTheStoreHolds: the list the billing page and
// the revenue board read is every subscription in the org's namespace, filtered
// only by what the caller asked.
func TestListSubscriptions_ReadsTheRowsTheStoreHolds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-list")
	db := datastore.New(org.Namespaced(ctx))
	if _, err := RecordSubscription(ctx, org, recordDev(t, db)); err != nil {
		t.Fatalf("record: %v", err)
	}
	liveSub(t, db, "other", "dev", "square")

	for _, tc := range []struct {
		user, status string
		want         int
	}{
		{"", "", 2}, {"acme", "", 1}, {"acme", "active", 1}, {"acme", "canceled", 0},
	} {
		rows, err := ListSubscriptions(ctx, org, tc.user, tc.status)
		if err != nil || len(rows) != tc.want {
			t.Errorf("user=%q status=%q: rows=%d err=%v, want %d", tc.user, tc.status, len(rows), err, tc.want)
		}
	}
	views, err := Subscriptions(ctx, org, "acme", "")
	if err != nil || len(views) != 1 || views[0].MRRCents <= 0 || views[0].Status != "active" {
		t.Fatalf("the billing page's view: %+v err=%v", views, err)
	}
}
