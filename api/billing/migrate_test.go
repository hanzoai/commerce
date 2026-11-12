package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/customerprice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// heldOn writes a live, card-paid subscription for subject on slug as the store
// holds one: the snapshot is the plan's catalog row at its interval, then edit.
func heldOn(t *testing.T, db *datastore.Datastore, subject, slug string, interval types.Interval, edit func(*subscription.Subscription)) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, slug)
	if err != nil {
		t.Fatalf("resolve %s: %v", slug, err)
	}
	if interval == types.Yearly {
		p, err = planAtInterval(p, types.Yearly)
		if err != nil {
			t.Fatalf("%s by the year: %v", slug, err)
		}
	}
	s := subscription.New(db)
	s.UserId, s.ProviderType, s.Quantity = subject, "square", 1
	s.Plan, s.PlanId, s.Status = *p, p.Id(), subscription.Active
	s.Start, s.PeriodStart = time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -10)
	s.PeriodEnd = s.PeriodStart.AddDate(0, 1, 0)
	if interval == types.Yearly {
		s.PeriodEnd = s.PeriodStart.AddDate(1, 0, 0)
	}
	if edit != nil {
		edit(s)
	}
	if err := s.Create(); err != nil {
		t.Fatalf("hold %s: %v", slug, err)
	}
	return s
}

// The table the owner set: every retired rung moves onto the dearest rung on sale
// listed at or below what its holder pays, or the rung the catalog names as its
// replacement; nobody pays more, a price staff agreed below the new list is kept,
// and nobody is moved onto Unlimited, which states no caps.
func TestMigrateMovesEveryRetiredRungOntoTheLadder(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := payerOrg(t, ctx, "acme")
	db := datastore.New(org.Namespaced(ctx))

	type want struct {
		to                string
		fromCents, toCent int64
		agreed            int64
	}
	cases := map[string]want{}
	hold := func(subject, slug string, interval types.Interval, w want, edit func(*subscription.Subscription)) {
		heldOn(t, db, subject, slug, interval, edit)
		cases[subject] = w
	}
	hold("acme/dev", "dev", types.Monthly, want{"pro-3x", 2000, 1500, 0}, nil)
	hold("acme/max-5x", "max-5x", types.Monthly, want{"max-15x", 10000, 9500, 0}, nil)
	hold("acme/max-20x", "max-20x", types.Monthly, want{"ultra", 20000, 19500, 0}, nil)
	hold("acme/max-50x", "max-50x", types.Monthly, want{"ultimate", 50000, 49500, 0}, nil)
	hold("acme/old-go", "go", types.Monthly, want{"go", 900, 800, 0}, func(s *subscription.Subscription) { s.Plan.Price = 900 })
	// The renamed and long-retired slugs as production holds them: archived rows of
	// the plan authority (scripts/ladder-writes.sh archives pro-2x and max-10x; the
	// seed archived max when the catalog dropped it).
	for slug, cents := range map[string]int64{"pro-2x": 1500, "max-10x": 9500, "max": 9900} {
		authorityPlan(t, ctx, slug, "personal", plan.StatusArchived, cents)
	}
	hold("acme/pro-2x", "pro-2x", types.Monthly, want{"pro-3x", 1500, 1500, 0}, nil)
	hold("acme/max-10x", "max-10x", types.Monthly, want{"max-15x", 9500, 9500, 0}, nil)
	hold("acme/max", "max", types.Monthly, want{"max-15x", 9900, 9500, 0}, nil)
	hold("acme/max-999", "max", types.Monthly, want{"ultimate", 99900, 49500, 0}, func(s *subscription.Subscription) { s.Plan.Price = 99900 })
	hold("acme/agreed-low", "max-20x", types.Monthly, want{"ultra", 15000, 15000, 15000}, func(s *subscription.Subscription) { s.CustomerCents = 15000 })
	hold("acme/agreed-high", "max-20x", types.Monthly, want{"ultra", 19900, 19500, 0}, func(s *subscription.Subscription) { s.CustomerCents = 19900 })
	// A yearly holder is placed by its rung's monthly list price, so it keeps the
	// multiple it bought; a year's agreed price is kept as its month.
	hold("acme/dev-year", "dev", types.Yearly, want{"pro-3x", 20000, 1500, 0}, nil)
	hold("acme/max-5x-year", "max-5x", types.Yearly, want{"max-15x", 100000, 9500, 0}, nil)
	hold("acme/max-50x-year", "max-50x", types.Yearly, want{"ultimate", 500000, 49500, 0}, nil)
	hold("acme/max-50x-agreed", "max-50x", types.Yearly, want{"ultimate", 200000, 16666, 16666}, func(s *subscription.Subscription) { s.CustomerCents = 200000 })

	// Left as they are: a rung on sale at its price, a private plan made for one
	// customer, a personal plan an admin made and then archived, a team, the free
	// plan, a cancelled row, a trial, and a merchant's customer on the merchant's own
	// plan that wears a retired Hanzo slug.
	authorityPlan(t, ctx, "patron", "personal", plan.StatusPrivate, 99500)
	authorityPlan(t, ctx, "bespoke", "personal", plan.StatusArchived, 150000)
	stays := map[string]string{"acme/on-go": "go", "acme/patron": "patron", "acme/bespoke": "bespoke", "acme/team": "team",
		"acme/free": "free", "acme/gone": "dev", "acme/trial": "dev"}
	for subject, slug := range stays {
		heldOn(t, db, subject, slug, types.Monthly, func(s *subscription.Subscription) {
			switch subject {
			case "acme/gone":
				s.Status, s.Canceled = subscription.Canceled, true
			case "acme/trial":
				s.Status, s.TrialStart, s.TrialEnd = subscription.Trialing, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 6)
			}
		})
	}
	merchant := plan.New(db)
	merchant.Slug, merchant.Name, merchant.Category, merchant.Price = "dev", "Merchant's dev kit", "personal", 2000
	if err := merchant.Create(); err != nil {
		t.Fatalf("merchant plan: %v", err)
	}
	heldOn(t, db, "acme/merchant-customer", "dev", types.Monthly, func(s *subscription.Subscription) { s.PlanId = merchant.Id() })
	stays["acme/merchant-customer"] = "dev"
	before := map[string]time.Time{}
	for _, s := range subsOf(t, db, "acme/dev-year") {
		before["acme/dev-year"] = s.PeriodEnd
	}

	m, err := MigrateRetired(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(m.Moved) != len(cases) || len(m.Errors) != 0 {
		t.Fatalf("moved %d (errors %v), want %d: %+v", len(m.Moved), m.Errors, len(cases), m.Moved)
	}
	for _, mv := range m.Moved {
		w, ok := cases[mv.Subject]
		if !ok {
			t.Errorf("%s was moved and should not have been", mv.Subject)
			continue
		}
		if mv.To != w.to || mv.FromCents != w.fromCents || mv.ToCents != w.toCent {
			t.Errorf("%s moved %s (%d) -> %s (%d), want -> %s at %d from %d", mv.Subject, mv.From, mv.FromCents, mv.To, mv.ToCents, w.to, w.toCent, w.fromCents)
		}
		s := subsOf(t, db, mv.Subject)[0]
		if subscriptionSlug(s) != w.to || s.Plan.Interval != types.Monthly || s.CustomerCents != w.agreed {
			t.Errorf("%s holds %s by the %s at an agreed %d, want %s monthly at %d", mv.Subject, subscriptionSlug(s), s.Plan.Interval, s.CustomerCents, w.to, w.agreed)
		}
		if mv.FromInterval == "month" && mv.ToCents > mv.FromCents {
			t.Errorf("%s pays more a month after the move: %d -> %d", mv.Subject, mv.FromCents, mv.ToCents)
		}
		if loadAutoRecharge(db, mv.Subject) != nil {
			t.Errorf("the move wrote %s a reload rule", mv.Subject)
		}
	}
	for subject, slug := range stays {
		if s := subsOf(t, db, subject)[0]; subscriptionSlug(s) != slug {
			t.Errorf("%s was moved onto %s, want it left on %s", subject, subscriptionSlug(s), slug)
		}
	}
	for _, s := range subsOf(t, db, "acme/max-999") {
		if subscriptionSlug(s) == "unlimited" {
			t.Fatal("a holder was moved onto Unlimited")
		}
	}
	// A yearly holder keeps the year it paid for.
	if s := subsOf(t, db, "acme/dev-year")[0]; !s.PeriodEnd.Equal(before["acme/dev-year"]) {
		t.Errorf("the yearly holder's period ends %v, want the year it paid for, %v", s.PeriodEnd, before["acme/dev-year"])
	}
	// The agreed price kept is recorded for the rung moved onto.
	r := customerprice.New(db)
	if err := r.Get(db.NewKey(r.Kind(), customerprice.ID("acme/agreed-low", "ultra", "month"), 0, nil)); err != nil || r.Cents != 15000 {
		t.Errorf("the agreed price on ultra: %+v %v, want 15000 recorded", r, err)
	}
	// Each move is audited, and a second run moves nobody.
	var events []*billingevent.BillingEvent
	if _, err := billingevent.Query(db).Filter("Type=", "subscription.migrated").GetAll(&events); err != nil || len(events) != len(cases) {
		t.Fatalf("audit records %d (%v), want %d", len(events), err, len(cases))
	}
	for _, e := range events {
		if e.Data["from"] == nil || e.Data["to"] == nil || e.Data["fromCents"] == nil || e.Data["toCents"] == nil {
			t.Errorf("an audit record names %v, want the plans and prices before and after", e.Data)
		}
	}
	again, err := MigrateRetired(ctx)
	if err != nil || len(again.Moved) != 0 {
		t.Fatalf("a second run moved %d (%v), want none", len(again.Moved), err)
	}
	if armedAt(t, ctx, org, "acme/dev") {
		t.Fatal("the migration switched a reload on")
	}
}

// The plan switches at the move and its price at the next renewal: the period in
// hand was paid at the old price, and the renewal charges the rung moved onto.
func TestAMigratedHolderRenewsAtTheNewPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := payerOrg(t, ctx, "mig-renew")
	m := squareMock("cust_mr", "ccof_mr", "sqpay_mr")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, org.Name, "max-20x", "ccof_mr", "cust_mr")
	if mv, err := MigrateRetired(ctx); err != nil || len(mv.Moved) != 1 {
		t.Fatalf("migrate: %+v %v", mv, err)
	}
	if tier, _ := deriveTier(db, org.Name, org.TestMode()); tier != "pro" {
		t.Fatalf("after the move the holder's tier is %q, want pro", tier)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success || out[0].AmountCents != 19500 {
		t.Fatalf("the renewal after the move: %+v, want ultra's 19500 on the card", out)
	}
	if s := reload(t, db, sub.Id()); subscriptionSlug(s) != "ultra" || s.Plan.Price != currency.Cents(19500) {
		t.Fatalf("after the renewal the row holds %s at %d, want ultra at 19500", subscriptionSlug(s), s.Plan.Price)
	}
}

// A comp moves as a comp: onto the rung at nothing, with the price it lists at
// kept beside it.
func TestAMigratedCompStaysAComp(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := payerOrg(t, ctx, "mig-comp")
	db := datastore.New(org.Namespaced(ctx))
	heldOn(t, db, "mig-comp", "dev", types.Monthly, func(s *subscription.Subscription) {
		s.Type, s.ProviderType = subscription.External, processorComp
		s.Metadata = map[string]interface{}{"listCents": int64(2000)}
		s.Plan.Price = 0
	})
	if mv, err := MigrateRetired(ctx); err != nil || len(mv.Moved) != 1 || mv.Moved[0].To != "pro-3x" {
		t.Fatalf("migrate: %+v %v, want the comp moved onto pro-3x", mv, err)
	}
	s := subsOf(t, db, "mig-comp")[0]
	if s.Plan.Price != 0 || metaInt64(s.Metadata, "listCents", 0) != 1500 || s.ProviderType != processorComp {
		t.Fatalf("the comp holds %d, lists %d, by %s; want 0, 1500, comp", s.Plan.Price, metaInt64(s.Metadata, "listCents", 0), s.ProviderType)
	}
}

// A coupon the holder was granted for the rung it held keeps applying on the rung
// it is moved onto: a max-5x holder at 50% off for six months renews on max-15x at
// half of $95, not at $95.
func TestAMigratedHolderKeepsItsCoupon(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := payerOrg(t, ctx, "mig-coupon")
	withFakeSquare(t, squareMock("cust_mc", "ccof_mc", "sqpay_mc"))
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, org.Name, "max-5x", "ccof_mc", "cust_mc")
	row := reload(t, db, sub.Id())
	engine.Grant(row, engine.Coupon{Code: "HALF6", Percent: 50, Months: 6}, []string{"max-5x"}, row.PeriodStart)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	if mv, err := MigrateRetired(ctx); err != nil || len(mv.Moved) != 1 || mv.Moved[0].To != "max-15x" {
		t.Fatalf("migrate: %+v %v, want max-5x moved onto max-15x", mv, err)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success || out[0].AmountCents != 4750 {
		t.Fatalf("the renewal after the move: %+v, want half of max-15x's 9500", out)
	}
}
