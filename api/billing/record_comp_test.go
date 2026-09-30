package billing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A comp: a catalog plan a SuperAdmin grants at no charge. It confers the plan —
// its tier, its seats — and nothing money-shaped: no draw, no invoice, no sale, no
// MRR, no allotment. It renews for nothing until it is canceled.

const compSubject = "acme"

// compIn is the comp staff grant: Team, two seats, one period starting today.
func compIn() RecordIn {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	return RecordIn{
		Subject:     compSubject,
		PlanID:      "team",
		Quantity:    2,
		PeriodStart: start,
		PeriodEnd:   start.AddDate(0, 1, 0),
		Processor:   "comp",
		Terms:       "owner request 2026-09-29: comp Team for acme",
		Actor:       "admin/z",
	}
}

func compSetup(t *testing.T, ctx context.Context, orgName string) (*organization.Organization, *datastore.Datastore) {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg(orgName)
	return org, datastore.New(org.Namespaced(ctx))
}

func invoicesOf(t *testing.T, db *datastore.Datastore, subject string) []*billinginvoice.BillingInvoice {
	t.Helper()
	out := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", subject).GetAll(&out); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	return out
}

// collector is an analytics collector that keeps every event posted to it.
type collector struct {
	mu     sync.Mutex
	bodies []string
}

func newCollector(t *testing.T) (*collector, *events.Client) {
	t.Helper()
	c := &collector{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies = append(c.bodies, string(b))
		c.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return c, events.NewClient(srv.URL)
}

// naming counts the events that name id.
func (c *collector) naming(id string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, b := range c.bodies {
		if strings.Contains(b, id) {
			n++
		}
	}
	return n
}

// await waits until an event naming id has arrived.
func (c *collector) await(t *testing.T, id string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c.naming(id) > 0 {
			return
		}
	}
	t.Fatalf("no event naming %s reached the collector", id)
}

func TestRecordComp_ConfersThePlanAndMovesNoMoney(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-open")
	deposit(t, db, compSubject, 50000)
	g := grant(t, db, compSubject, 3000)
	col, ev := newCollector(t)

	in := compIn()
	in.Events = ev
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Outcome != RecordCreated || got.Replaced != nil {
		t.Fatalf("recorded %+v, want a created row that replaced nothing", got)
	}

	s := reload(t, db, got.Subscription.ID)
	if s.Status != subscription.Active || string(s.Type) != "comp" || s.ProviderType != "comp" || s.DefaultPaymentMethod != "" {
		t.Fatalf("status=%q type=%q provider=%q method=%q, want an active comp that names nothing a charge could use",
			s.Status, s.Type, s.ProviderType, s.DefaultPaymentMethod)
	}
	if s.Plan.Slug != "team" || s.Quantity != 2 || s.Plan.Price != 0 || s.Plan.ContactSales || s.CurrentInvoiceId != "" {
		t.Fatalf("plan %q x%d at %d (contactSales %v, invoice %q), want team for two seats at $0 with no invoice",
			s.Plan.Slug, s.Quantity, s.Plan.Price, s.Plan.ContactSales, s.CurrentInvoiceId)
	}
	if !s.PeriodStart.Equal(in.PeriodStart) || !s.PeriodEnd.Equal(in.PeriodEnd) || !s.TrialEnd.IsZero() {
		t.Fatalf("period %s..%s trial %s, want the granted period and no trial", s.PeriodStart, s.PeriodEnd, s.TrialEnd)
	}
	if s.Metadata["collection"] != "comp" || s.Metadata["terms"] != in.Terms || s.Metadata["actor"] != in.Actor {
		t.Fatalf("metadata %#v, want the comp, its terms and who granted it", s.Metadata)
	}

	// No money moved: no invoice, the balance and the credit are untouched.
	if invs := invoicesOf(t, db, compSubject); len(invs) != 0 {
		t.Fatalf("a comp opened %d invoice(s)", len(invs))
	}
	if b := walletOf(t, ctx, org, compSubject); b != 50000 {
		t.Fatalf("balance = %d, want the 50000 it held", b)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != 3000 {
		t.Fatalf("credit grant holds %d (%v), want the 3000 it held", left.RemainingCents, err)
	}

	// It is no revenue.
	if m := SubscriptionMRRCents(s); m != 0 || got.Subscription.MRRCents != 0 {
		t.Fatalf("MRR = %d (answered %d), want 0", m, got.Subscription.MRRCents)
	}
	rows, err := Subscriptions(ctx, org, compSubject, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("subscriptions = %+v, %v; want the comp", rows, err)
	}
	if r := rows[0]; r.ProviderType != "comp" || r.Settled != "comp" || r.MRRCents != 0 || r.Plan.Price != 0 || r.Plan.Name != "Team" {
		t.Fatalf("the billing page reads %+v, want Team marked comp at $0 with no MRR", r)
	}

	// It confers the plan: the tier, the plan served, and it is the plan held.
	if tr, err := deriveTier(db, compSubject, org.TestMode()); err != nil || tr != tier.Pro {
		t.Fatalf("tier = %q, %v; want pro", tr, err)
	}
	view, err := ReadTier(ctx, org, compSubject, tier.Pro)
	if err != nil || view.Plan != "team" {
		t.Fatalf("tier view plan = %+v, %v; want team", view, err)
	}
	if held := billingSubscription(db, compSubject, org.TestMode()); held == nil || held.Id() != s.Id() {
		t.Fatal("the comp must be the plan the subject holds, or a second plan opens beside it")
	}
	// And no money hangs off it: it is not payment-backed and anchors no allotment.
	if subscriptionPaymentBacked(s) {
		t.Fatal("a comp reads as payment-backed")
	}
	if p := planForGrant(nil, db, compSubject, "", org.TestMode()); p != "" {
		t.Fatalf("the allotment would mint for %q; a comp anchors none", p)
	}

	// No sale event. A payment recorded after it, on the same collector, is
	// seen, so silence about the comp is not a dead collector.
	paid := recordDev(t, db)
	paid.Subject, paid.Events = "beta", ev
	sale, err := RecordSubscription(ctx, org, paid)
	if err != nil {
		t.Fatalf("record the control sale: %v", err)
	}
	col.await(t, sale.Subscription.ID)
	if n := col.naming(s.Id()); n != 0 {
		t.Fatalf("the comp reached the analytics collector %d time(s)", n)
	}
}

func TestRecordComp_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-refuse")
	authorityPlan(t, ctx, "retainer", "retainer", plan.StatusPrivate, 2500)
	deposit(t, db, compSubject, 50000)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	for name, tc := range map[string]struct {
		edit func(*RecordIn)
		says string
	}{
		"a price":                {func(in *RecordIn) { in.PriceCents = 5000 }, "priceCents"},
		"a negative price":       {func(in *RecordIn) { in.PriceCents = -1 }, "priceCents"},
		"a payment reference":    {func(in *RecordIn) { in.Reference = map[string]string{"payment": "pay_1"} }, "reference"},
		"no terms":               {func(in *RecordIn) { in.Terms = " " }, "terms"},
		"no actor":               {func(in *RecordIn) { in.Actor = "" }, "actor"},
		"one seat of team":       {func(in *RecordIn) { in.Quantity = 1 }, "seats"},
		"a private plan":         {func(in *RecordIn) { in.PlanID, in.Quantity = "retainer", 1 }, "catalog"},
		"a free plan":            {func(in *RecordIn) { in.PlanID, in.Quantity = "free", 1 }, "free"},
		"a period gone by":       {func(in *RecordIn) { in.PeriodStart, in.PeriodEnd = today.AddDate(0, -2, 0), today.AddDate(0, -1, 0) }, "period"},
		"a period of two months": {func(in *RecordIn) { in.PeriodEnd = today.AddDate(0, 2, 0) }, "period"},
	} {
		in := compIn()
		tc.edit(&in)
		_, err := RecordSubscription(ctx, org, in)
		if err == nil || !IsSaleRefused(err) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: err = %v; want a refusal that names %q", name, err, tc.says)
		}
	}
	if subs := subsOf(t, db, compSubject); len(subs) != 0 {
		t.Fatalf("refused comps wrote %d subscription(s)", len(subs))
	}
	if b := walletOf(t, ctx, org, compSubject); b != 50000 {
		t.Fatalf("balance = %d after refusals, want 50000", b)
	}
}

func TestRecordComp_RenewsForNothingUntilCanceled(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-renew")
	deposit(t, db, compSubject, 50000)
	got, err := RecordSubscription(ctx, org, compIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	id := got.Subscription.ID

	// The cycle takes it once its period ends and rolls it on, collecting nothing.
	due(t, db, id)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || out[0].AmountCents != 0 || out[0].InvoiceId != "" {
		t.Fatalf("cycle = %+v, want the comp rolled on with no invoice and no money", out)
	}
	s := reload(t, db, id)
	now := time.Now()
	if s.Status != subscription.Active || s.PeriodStart.After(now) || !s.PeriodEnd.After(now) {
		t.Fatalf("after renewal the comp is %s on %s..%s, want active on the period running now", s.Status, s.PeriodStart, s.PeriodEnd)
	}
	if invs := invoicesOf(t, db, compSubject); len(invs) != 0 {
		t.Fatalf("renewing a comp opened %d invoice(s)", len(invs))
	}
	if b := walletOf(t, ctx, org, compSubject); b != 50000 {
		t.Fatalf("balance = %d, want the 50000 it held", b)
	}

	// Nothing the engine is handed is ever asked: not prepaid money, not a card.
	due(t, db, id)
	row := reload(t, db, id)
	burn, charged := &countingPrepaid{}, 0
	charge := func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		charged++
		return "ref", nil
	}
	inv, res, err := engine.RenewSubscription(ctx, db, row, burn, charge)
	if err != nil || inv != nil || res == nil || !res.Success || burn.calls != 0 || charged != 0 {
		t.Fatalf("renew: invoice=%v result=%+v err=%v; asked prepaid %d time(s), charged %d time(s); want rolled on with nothing asked",
			inv != nil, res, err, burn.calls, charged)
	}
	if row.Status != subscription.Active || !row.PeriodEnd.After(time.Now()) {
		t.Fatalf("the comp is %s through %s, want active and never past due", row.Status, row.PeriodEnd)
	}

	// Revoking it is the one cancel, and it ends the plan.
	if _, err := CancelSubscription(ctx, org, AnyHolder, id, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if tr, _ := deriveTier(db, compSubject, org.TestMode()); tr != tier.Free {
		t.Fatalf("tier after revoking = %q, want free", tr)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("a revoked comp renewed: %+v", out)
	}
}

func TestRecordComp_CanceledAtItsPeriodEndEndsThen(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-end")
	got, err := RecordSubscription(ctx, org, compIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := CancelSubscription(ctx, org, AnyHolder, got.Subscription.ID, true); err != nil {
		t.Fatalf("cancel at period end: %v", err)
	}
	due(t, db, got.Subscription.ID)
	renewDue(ctx, org, "")
	if s := reload(t, db, got.Subscription.ID); s.Status != subscription.Canceled {
		t.Fatalf("a comp canceled at its period end is %s after it, want canceled", s.Status)
	}
}

func TestRecordComp_IsTheOnePlanHeld(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-held")

	paid := recordDev(t, db)
	held, err := RecordSubscription(ctx, org, paid)
	if err != nil {
		t.Fatalf("record the held plan: %v", err)
	}

	// Over a plan the subject holds, a comp must name it.
	if _, err := RecordSubscription(ctx, org, compIn()); err == nil || !strings.Contains(err.Error(), "replaces") {
		t.Fatalf("a comp beside a held plan: %v; want a refusal that says to name it in replaces", err)
	}
	in := compIn()
	in.Replaces = held.Subscription.ID
	comp, err := RecordSubscription(ctx, org, in)
	if err != nil || comp.Outcome != RecordCreated || comp.Replaced == nil || comp.Replaced.Status != string(subscription.Canceled) {
		t.Fatalf("comp over the held plan: %+v, %v; want it created and the held plan ended", comp, err)
	}

	// A retry is the comp already standing.
	again, err := RecordSubscription(ctx, org, compIn())
	if err != nil || again.Outcome != RecordUnchanged || again.Subscription.ID != comp.Subscription.ID {
		t.Fatalf("retry: %+v, %v; want the same comp unchanged", again, err)
	}

	// More seats is a new comp that takes this one over.
	more := compIn()
	more.Quantity = 3
	if _, err := RecordSubscription(ctx, org, more); err == nil {
		t.Fatal("a second comp opened beside the first")
	}
	more.Replaces = comp.Subscription.ID
	bigger, err := RecordSubscription(ctx, org, more)
	if err != nil || bigger.Subscription.Quantity != 3 || bigger.Replaced == nil || bigger.Replaced.ID != comp.Subscription.ID {
		t.Fatalf("three seats over two: %+v, %v", bigger, err)
	}

	// A payment for a plan takes the comp over only by naming it.
	pay := recordDev(t, db)
	if _, err := RecordSubscription(ctx, org, pay); err == nil || !strings.Contains(err.Error(), "comp") {
		t.Fatalf("a payment beside a comp: %v; want a refusal that names the comp", err)
	}
	pay.Replaces = bigger.Subscription.ID
	if rec, err := RecordSubscription(ctx, org, pay); err != nil || rec.Replaced == nil || rec.Replaced.ID != bigger.Subscription.ID {
		t.Fatalf("a payment over the comp: %+v, %v", rec, err)
	}

	live := 0
	for _, s := range subsOf(t, db, compSubject) {
		if s.Status == subscription.Active {
			live++
		}
	}
	if live != 1 {
		t.Fatalf("%d active plans, want exactly one", live)
	}
}

// A comp's plan and seats are the grant; the subscription PATCH changes neither,
// for the customer or for a platform principal.
func TestRecordComp_ThePlanAndSeatsAreNotPatched(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "acme")
	got, err := RecordSubscription(ctx, org, compIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	id := got.Subscription.ID
	if w := invokeSubPatch(org, ctx, c1OrgAdmin, id, `{"quantity":50}`); w.StatusCode < 400 {
		t.Fatalf("an org admin moved a comp to 50 seats: %d %s", w.StatusCode, bodyOf(w))
	}
	if w := invokeSubPatch(org, ctx, c1MintPrincipal, id, `{"planId":"max-20x"}`); w.StatusCode < 400 {
		t.Fatalf("a platform principal moved a comp onto max-20x: %d %s", w.StatusCode, bodyOf(w))
	}
	if s := reload(t, db, id); s.Quantity != 2 || s.Plan.Slug != "team" || s.Plan.Price != 0 {
		t.Fatalf("the comp is %q x%d at %d, want team x2 at $0", s.Plan.Slug, s.Quantity, s.Plan.Price)
	}
}

// The event backfill replays no comp: nothing was sold.
func TestBackfill_ReplaysNoComp(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := compSetup(t, ctx, "comp-backfill")
	got, err := RecordSubscription(ctx, org, compIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if planned := planSubscription(org.Name, reload(t, db, got.Subscription.ID)); len(planned) != 0 {
		t.Fatalf("backfill would replay %d event(s) for a comp", len(planned))
	}
}
