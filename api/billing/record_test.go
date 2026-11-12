package billing

import (
	"context"
	"strings"
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

// The recorded period is the one running now: a plan collected outside commerce
// lapses at its end, so a period already over would read as no plan at all.
var (
	recStart = time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -15)
	recEnd   = recStart.AddDate(0, 1, 0)
)

func recordGo(t *testing.T, db *datastore.Datastore) RecordIn {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, "go")
	if err != nil || p.Price <= 0 {
		t.Fatalf("the catalog must sell a paid go plan: %v (price %d)", err, p.Price)
	}
	return RecordIn{
		Subject:     "acme",
		PlanID:      "go",
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
	in := recordGo(t, db)

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
	if int64(s.Plan.Price) != in.PriceCents || s.Plan.Slug != "go" {
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

func TestRecordedPlanIsNeverChargedOrRenewedByTheProcessor(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-never")
	db := datastore.New(org.Namespaced(ctx))
	if _, err := RecordSubscription(ctx, org, recordGo(t, db)); err != nil {
		t.Fatalf("record: %v", err)
	}
	s := subsOf(t, db, "acme")[0]

	afterEnd := recEnd.Add(24 * time.Hour)
	if engine.IsDue(s, afterEnd) {
		t.Fatal("an externally collected plan answered due; the cycle would invoice and collect it")
	}
	charged := 0
	burn := &countingPrepaid{}
	charge := func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		charged++
		return "ref", nil
	}
	inv, _, err := engine.RenewSubscription(ctx, db, s, burn, charge)
	burned := burn.calls
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if inv != nil || burned != 0 || charged != 0 {
		t.Fatalf("renewal built invoice=%v, burned credits %d time(s), charged %d time(s); it must do none", inv != nil, burned, charged)
	}
	if !s.PeriodEnd.Equal(recEnd) || s.Status != subscription.Active {
		t.Fatalf("renewal moved the row to %s %s; only a recorded payment moves it", s.Status, s.PeriodEnd)
	}
}

func TestRecordSubscription_RetryIsUnchangedAndTheNextPaymentExtends(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-next")
	db := datastore.New(org.Namespaced(ctx))
	in := recordGo(t, db)
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

// A plan the catalog retires while a subject pays for it by wire still takes its
// next payment: the record extends the row it holds, priced from the row's own
// snapshot, as a card renewal of an archived plan bills. Opening the retired plan
// for anyone else, or moving onto it, is refused as a plan that is not there.
// Mutation: gate the extension on Assignable and the next period is refused.
func TestRecordSubscription_ARetiredPlanHeldTakesItsNextPayment(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("rec-retired")
	db := datastore.New(org.Namespaced(ctx))
	in := recordGo(t, db)
	in.Processor = "wire"
	if _, err := RecordSubscription(ctx, org, in); err != nil {
		t.Fatalf("record: %v", err)
	}
	archivePlan(t, ctx, "go")

	next := in
	next.PeriodStart, next.PeriodEnd = recEnd, recEnd.AddDate(0, 1, 0)
	next.Reference = map[string]string{"invoice": "inv:0-retired"}
	got, err := RecordSubscription(ctx, org, next)
	if err != nil || got.Outcome != RecordExtended {
		t.Fatalf("the next period of a retired plan held: outcome=%v err=%v, want extended", got, err)
	}
	short := next
	short.PeriodStart, short.PeriodEnd = next.PeriodEnd, next.PeriodEnd.AddDate(0, 1, 0)
	short.PriceCents--
	if _, err := RecordSubscription(ctx, org, short); !IsSaleRefused(err) || !strings.Contains(err.Error(), "costs 800 cents") {
		t.Fatalf("a short payment for the retired plan held: err=%v, want refused against its price", err)
	}
	// Taking the held row over onto the retired plan opens it anew, which only a
	// plan on sale or a private one may be.
	held := subsOf(t, db, in.Subject)[0]
	over := next
	over.PeriodStart, over.PeriodEnd = short.PeriodStart, short.PeriodEnd
	over.Replaces, over.Processor, over.PriceCents = held.Id(), "comp", 0
	over.Reference, over.Terms = map[string]string{"order": "comp-retired"}, "given by the owner"
	if _, err := RecordSubscription(ctx, org, over); !IsSaleNotFound(err) {
		t.Fatalf("a comp taking the held row over onto the retired plan: err=%v, want not found", err)
	}

	other := in
	other.Subject = "acme/new"
	if _, err := RecordSubscription(ctx, org, other); !IsSaleNotFound(err) {
		t.Fatalf("opening a retired plan for another subject: err=%v, want not found", err)
	}
	if n := len(subsOf(t, db, "acme/new")); n != 0 {
		t.Fatalf("the refused record opened %d subscription(s)", n)
	}
}

func TestRecordSubscription_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("rec-price")
	db := datastore.New(org.Namespaced(ctx))
	wrong := recordGo(t, db)
	wrong.PriceCents++
	if _, err := RecordSubscription(ctx, org, wrong); !IsSaleRefused(err) {
		t.Fatalf("a payment that is not the plan's price: err=%v, want refused", err)
	}
	if n := len(subsOf(t, db, wrong.Subject)); n != 0 {
		t.Fatalf("a refused record wrote %d row(s)", n)
	}

	unknown := recordGo(t, db)
	unknown.PlanID = "no-such-plan"
	if _, err := RecordSubscription(ctx, org, unknown); !IsSaleNotFound(err) {
		t.Fatalf("an unknown plan: err=%v, want not found", err)
	}

	noRef := recordGo(t, db)
	noRef.Reference = map[string]string{"invoice": " "}
	if _, err := RecordSubscription(ctx, org, noRef); !IsSaleRefused(err) {
		t.Fatalf("no reference: err=%v, want refused", err)
	}

	paying := moneyOrg("rec-paying")
	pdb := datastore.New(paying.Namespaced(ctx))
	liveSub(t, pdb, "acme", "go", "square")
	if _, err := RecordSubscription(ctx, paying, recordGo(t, pdb)); !IsSaleConflict(err) {
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
	if _, err := RecordSubscription(ctx, org, recordGo(t, db)); err != nil {
		t.Fatalf("record: %v", err)
	}
	liveSub(t, db, "other", "go", "square")

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

// Each subscription says how its period in hand was paid: a plan recorded as
// collected outside commerce names its processor, a plan paid from the balance says
// so, and a row nobody paid for says nothing.
func TestSubscriptionsSayHowThePeriodWasPaid(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-settled")
	withFakeSquare(t, squareMock("cust_1", "ccof_1", "sqpay_1"))
	db := datastore.New(org.Namespaced(ctx))
	paid := recordGo(t, db)
	paid.Subject = "acme-square"
	comp := compEnterprise("acme-comp")
	comp.PlanID = "go"
	for _, in := range []RecordIn{paid, comp} {
		if _, err := RecordSubscription(ctx, org, in); err != nil {
			t.Fatalf("record %s: %v", in.Processor, err)
		}
	}
	for subject, want := range map[string]string{"acme-square": "external:square", "acme-comp": "external:comp"} {
		rows, err := Subscriptions(ctx, org, subject, "")
		if err != nil || len(rows) != 1 || rows[0].Settled != want {
			t.Fatalf("%s: %+v, %v; want settled %q", subject, rows, err, want)
		}
	}
}
