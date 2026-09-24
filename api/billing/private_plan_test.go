package billing

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A private plan is made for one customer: a row in the plan authority, priced by
// agreement, that the catalog does not publish. Nobody can find it or buy it; staff
// put the customer on it by recording the payment; and once they have, it confers
// the paid tier it was sold at.

// authorityPlan writes one plan row the catalog does not publish, the way the admin
// CRUD writes it.
func authorityPlan(t *testing.T, ctx context.Context, slug, category, status string, cents int64) {
	t.Helper()
	p := plan.New(datastore.New(nscontext.WithNamespace(ctx, plan.Namespace)))
	p.Slug, p.Name, p.Category, p.Status = slug, slug, category, status
	p.Price, p.Currency = currency.Cents(cents), currency.USD
	p.Interval, p.IntervalCount = types.Monthly, 1
	p.AdminEdited = true
	if err := p.Create(); err != nil {
		t.Fatalf("create plan %s: %v", slug, err)
	}
}

func TestPlanStatus_PrivateIsAssignableAndNeverListed(t *testing.T) {
	p := plan.Plan{Status: plan.StatusPrivate}
	if p.Listed() || !p.Assignable() {
		t.Fatalf("private: listed=%v assignable=%v, want hidden and assignable", p.Listed(), p.Assignable())
	}
	for _, s := range []string{"", plan.StatusActive} {
		p.Status = s
		if !p.Assignable() {
			t.Errorf("%q is listed but not assignable", s)
		}
	}
	for _, s := range []string{plan.StatusDraft, plan.StatusArchived} {
		p.Status = s
		if p.Assignable() {
			t.Errorf("%q is assignable; only a listed or a private plan may be put on a customer", s)
		}
	}
}

func TestPrivatePlan_HiddenFromEveryoneButStaff(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	authorityPlan(t, ctx, "agency", "agency", plan.StatusPrivate, 20000)
	org := moneyOrg("webby")

	// Not in the public catalog, by list or by slug.
	rows, err := ReadPlans(ctx, "", nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	for _, r := range rows {
		if r.Slug == "agency" {
			t.Fatal("a private plan is in the public catalog")
		}
	}

	// No self-serve path opens it: not the mint principal's create, not the card.
	body := `{"userId":"webby","planId":"agency"}`
	if w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body); w.StatusCode != http.StatusNotFound {
		t.Fatalf("create on a private plan: %d %s, want 404", w.StatusCode, bodyOf(w))
	}
	if w := invokeSubscribeCard(org, ctx, `{"planId":"agency","sourceId":"cnon:card-nonce-ok"}`, nil); w.StatusCode != http.StatusNotFound {
		t.Fatalf("card on a private plan: %d %s, want 404", w.StatusCode, bodyOf(w))
	}

	// Staff record it.
	start := time.Now().UTC().Truncate(24 * time.Hour)
	in := RecordIn{
		Subject: "webby", PlanID: "agency", PriceCents: 20000,
		PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0),
		Processor: "wire", Reference: map[string]string{"payment": "FED2026A"},
	}
	rec, err := RecordSubscription(ctx, org, in)
	if err != nil || rec.Outcome != RecordCreated || rec.Subscription.Status != string(subscription.Active) {
		t.Fatalf("record: %+v, %v; want an active subscription", rec, err)
	}
	// Moving onto it by plan change is refused too: the move is self-serve.
	liveSub(t, datastore.New(org.Namespaced(ctx)), "other", "dev", "square")
	if w := invokeSubPatch(org, ctx, c1MintPrincipal, rec.Subscription.ID, `{"planId":"agency"}`); w.StatusCode != http.StatusNotFound {
		t.Fatalf("plan change onto a private plan: %d %s, want 404", w.StatusCode, bodyOf(w))
	}

	// The price check stays strict, and a draft is nobody's to assign.
	wrong := in
	wrong.Subject, wrong.PriceCents = "webby2", 19999
	if _, err := RecordSubscription(ctx, org, wrong); !IsSaleRefused(err) {
		t.Fatalf("a payment that is not the plan's price: err=%v, want refused", err)
	}
	authorityPlan(t, ctx, "unfinished", "agency", plan.StatusDraft, 20000)
	draft := in
	draft.Subject, draft.PlanID = "webby3", "unfinished"
	if _, err := RecordSubscription(ctx, org, draft); !IsSaleNotFound(err) {
		t.Fatalf("a draft plan: err=%v, want not found", err)
	}
}

// A plan only the authority holds confers the tier it was sold at, read from the
// subscription's own snapshot — on a row a payment opened, and on no other.
func TestPrivatePlan_ConfersThePaidTierItWasSoldAt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	authorityPlan(t, ctx, "agency", "agency", plan.StatusPrivate, 20000)
	org := moneyOrg("webby")
	db := datastore.New(org.Namespaced(ctx))

	start := time.Now().UTC().Truncate(24 * time.Hour)
	in := RecordIn{
		Subject: "webby", PlanID: "agency", PriceCents: 20000,
		PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0),
		Processor: "wire", Reference: map[string]string{"payment": "FED2026A"},
	}
	if _, err := RecordSubscription(ctx, org, in); err != nil {
		t.Fatalf("record: %v", err)
	}
	if tier, err := TierOf(ctx, org, "webby"); err != nil || tier != "pro" {
		t.Fatalf("tier=%q err=%v, want the paid tier the plan was sold at", tier, err)
	}
	// A retry is the same row, not a second one: the held plan is found.
	again, err := RecordSubscription(ctx, org, in)
	if err != nil || again.Outcome != RecordUnchanged {
		t.Fatalf("retry: %+v, %v; want unchanged", again, err)
	}
	if n := len(subsOf(t, db, "webby")); n != 1 {
		t.Fatalf("subscriptions=%d after a retry, want 1", n)
	}

	// The same kind of plan, on a row nobody paid for, confers nothing: the
	// creation gate reads the catalog, so it cannot vouch for this plan.
	authorityPlan(t, ctx, "studio", "team", plan.StatusActive, 5000)
	liveSub(t, db, "freeloader", "studio", "internal")
	if tier, _ := TierOf(ctx, org, "freeloader"); tier != "free" {
		t.Fatalf("a self-created row on an uncatalogued plan reads %q, want free", tier)
	}
	// And a plan the catalog publishes still answers from the catalog alone.
	liveSub(t, db, "comped", "max-5x", "manual_gift")
	if tier, _ := TierOf(ctx, org, "comped"); tier != "pro" {
		t.Fatalf("a comped catalog plan reads %q, want pro", tier)
	}
}

// uncatalogued writes a listed plan the catalog does not publish: one the admin
// CRUD added to the public page, priced and tiered by its own row.
func uncatalogued(t *testing.T, ctx context.Context, slug, category string, cents int64, contactSales bool) {
	t.Helper()
	p := plan.New(datastore.New(nscontext.WithNamespace(ctx, plan.Namespace)))
	p.Slug, p.Name, p.Category, p.Status = slug, slug, category, plan.StatusActive
	p.Price, p.Currency, p.ContactSales = currency.Cents(cents), currency.USD, contactSales
	p.Interval, p.IntervalCount = types.Monthly, 1
	p.AdminEdited = true
	if err := p.Create(); err != nil {
		t.Fatalf("create plan %s: %v", slug, err)
	}
}

// renewedFree opens the free plan as an org admin and renews it once. Its invoice
// is $0, so the collection succeeds and the row reads invoiced — with no money.
func renewedFree(t *testing.T, ctx context.Context, org *organization.Organization, subject string) *subscription.Subscription {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	w := invokeSub(org, ctx, c1OrgAdmin, CreateBillingSubscription, `{"userId":"`+subject+`","planId":"free"}`)
	if w.StatusCode != http.StatusCreated {
		t.Fatalf("open free: %d %s", w.StatusCode, bodyOf(w))
	}
	var id string
	for _, s := range subsOf(t, db, subject) {
		if s.ProviderType != "bundle" {
			id = s.Id()
		}
	}
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("load: %v", err)
	}
	s.PeriodStart, s.PeriodEnd = time.Now().AddDate(0, -1, -1), time.Now().Add(-time.Hour)
	if _, res, err := engine.RenewSubscription(ctx, db, s, BurnCredits, nil); err != nil || !res.Success {
		t.Fatalf("renew: %+v, %v", res, err)
	}
	if err := s.Update(); err != nil || !subscriptionPaymentBacked(s) {
		t.Fatalf("a $0 renewal left the row unbacked (err=%v); the precondition this test needs is gone", err)
	}
	return s
}

// A move onto a paid plan the catalog does not publish is staff's, whatever row it
// starts from. The catalog cannot score what that plan covers, the tier reads it
// from the snapshot the move writes, and the payment on the row was for the plan it
// leaves — so a customer moving themselves would take a tier nobody paid for.
func TestPrivatePlan_MoveOntoAnUncataloguedPlanIsStaffs(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uncatalogued(t, ctx, "scale", "enterprise", 500000, false)
	org := moneyOrg("acme")
	db := datastore.New(org.Namespaced(ctx))

	dev, _ := resolveSubscriptionPlan(db, "dev")
	start := time.Now().UTC().Truncate(24 * time.Hour)
	recorded, err := RecordSubscription(ctx, org, RecordIn{
		Subject: "acme/wired", PlanID: "dev", PriceCents: int64(dev.Price),
		PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0),
		Processor: "wire", Reference: map[string]string{"payment": "FED2026B"},
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	rows := map[string]string{
		"acme/card":  liveSub(t, db, "acme/card", "dev", "square").Id(),
		"acme/wired": recorded.Subscription.ID,
		"acme/free":  renewedFree(t, ctx, org, "acme/free").Id(),
	}
	for subject, id := range rows {
		before, _ := TierOf(ctx, org, subject)
		if w := invokeSubPatch(org, ctx, c1OrgAdmin, id, `{"planId":"scale"}`); w.StatusCode != http.StatusForbidden {
			t.Errorf("%s: org admin moved onto an uncatalogued plan: %d %s, want 403", subject, w.StatusCode, bodyOf(w))
		}
		if after, _ := TierOf(ctx, org, subject); after != before {
			t.Errorf("%s: tier %q -> %q across a refused move", subject, before, after)
		}
	}

	// Staff may.
	if w := invokeSubPatch(org, ctx, c1MintPrincipal, rows["acme/card"], `{"planId":"scale"}`); w.StatusCode != http.StatusOK {
		t.Fatalf("staff move: %d %s, want 200", w.StatusCode, bodyOf(w))
	}
	if tier, _ := TierOf(ctx, org, "acme/card"); tier != "enterprise" {
		t.Fatalf("after the staff move the row reads %q, want enterprise", tier)
	}
}

// Opening a paid plan the catalog does not publish is staff's too. Left open, the
// row starts internal, and its first renewal marks it invoiced: a contact-sales
// plan invoices $0, so that collection succeeds with no money and the row confers
// the tier. A free uncatalogued plan stays self-serve.
func TestPrivatePlan_OpeningAnUncataloguedPaidPlanIsStaffs(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uncatalogued(t, ctx, "bespoke", "enterprise", 0, true)
	uncatalogued(t, ctx, "studio", "team", 5000, false)
	uncatalogued(t, ctx, "hobby", "personal", 0, false)
	org := moneyOrg("acme")

	for _, slug := range []string{"bespoke", "studio"} {
		body := `{"userId":"acme","planId":"` + slug + `"}`
		if w := invokeSub(org, ctx, c1OrgAdmin, CreateBillingSubscription, body); w.StatusCode != http.StatusForbidden {
			t.Errorf("org admin opened %s: %d %s, want 403", slug, w.StatusCode, bodyOf(w))
		}
		if w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body); w.StatusCode != http.StatusCreated {
			t.Errorf("staff opening %s: %d %s, want 201", slug, w.StatusCode, bodyOf(w))
		}
	}
	if w := invokeSub(org, ctx, c1OrgAdmin, CreateBillingSubscription, `{"userId":"acme/h","planId":"hobby"}`); w.StatusCode != http.StatusCreated {
		t.Errorf("org admin opening a free uncatalogued plan: %d %s, want 201", w.StatusCode, bodyOf(w))
	}
}

// A slug the catalog publishes still answers from the catalog alone, on any row:
// the uncatalogued branch never reaches one.
func TestPrivatePlan_CatalogTiersAreUnchanged(t *testing.T) {
	slugs := make([]string, 0, len(catalog)+len(successor))
	for i := range catalog {
		slugs = append(slugs, catalog[i].Slug)
	}
	for old := range successor {
		slugs = append(slugs, old)
	}
	for _, slug := range slugs {
		for _, provider := range []string{"internal", "square", "manual_gift"} {
			for _, price := range []currency.Cents{0, 1, 999999} {
				s := &subscription.Subscription{ProviderType: provider, PlanId: slug}
				s.Plan.Slug, s.Plan.Price, s.Plan.Category = slug, price, "enterprise"
				if got, want := activeTier(s), tierForActivePaidSlug(slug); got != want {
					t.Errorf("%s on %s at %d: %q, want the catalog's %q", slug, provider, price, got, want)
				}
			}
		}
	}
}
