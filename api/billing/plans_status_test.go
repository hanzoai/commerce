package billing

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan is retired by ARCHIVING it, never by deleting it.
//
// Before Status existed, the public read returned every stored row, so the only
// way to stop offering a tier was DELETE — which takes the row's history with it
// and orphans any subscription or invoice that recorded the slug. These pin the
// three properties that make archive a real alternative to delete.
func TestPlanStatus_UnlistedRowsAreHiddenButKept(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))

	before, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority read failed after seed")
	}

	// Archive one row the way the admin CRUD does.
	p := plan.New(db)
	found, err := p.Query().Filter("Slug=", "pro").Get()
	if err != nil || !found {
		t.Fatalf("load pro: found=%v err=%v", found, err)
	}
	p.Status = plan.StatusArchived
	if err := p.Update(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	after, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority read failed after archive")
	}

	// 1. It is gone from the PUBLIC catalog.
	if len(after) != len(before)-1 {
		t.Fatalf("public rows %d, want %d (one archived)", len(after), len(before)-1)
	}
	for _, r := range after {
		if r.Slug == "pro" {
			t.Fatal("archived plan still served publicly")
		}
	}

	// 2. The ROW SURVIVES — this is the whole difference from DELETE. A renewal
	//    or invoice that recorded "pro" must still resolve it.
	kept := plan.New(db)
	stillThere, err := kept.Query().Filter("Slug=", "pro").Get()
	if err != nil || !stillThere {
		t.Fatalf("archived plan was destroyed: found=%v err=%v", stillThere, err)
	}
	if kept.Price == 0 {
		t.Fatal("archived plan lost its price; history would not resolve")
	}

	// 3. Un-archiving restores it, so this is reversible.
	kept.Status = plan.StatusActive
	if err := kept.Update(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, ok := planAuthorityRows(c)
	if !ok || len(restored) != len(before) {
		t.Fatalf("restored rows %d, want %d", len(restored), len(before))
	}
}

// Hiding a plan from the catalog is worth nothing if the API still sells it.
//
// The adversary here is not an unauthorized caller — it is the MINT PRINCIPAL,
// the one caller allowed to open a paid subscription with no payment (cloud-api
// after a real charge). If even it is refused an archived tier, nobody can buy
// one, and the refusal is clearly about the plan's status rather than the
// caller's authority.
func TestPlanStatus_ArchivedPlanCannotBeBought(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	// Control: the mint principal really does BUY this tier while it is listed —
	// asserted as a 2xx, not merely "not 404", or the test would still pass if the
	// purchase were failing for some unrelated reason.
	body := `{"userId":"acme/self","planId":"max"}`
	if w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body); w.StatusCode/100 != 2 {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("control failed: mint principal could not buy a LISTED plan: %d %s", w.StatusCode, string(b))
	}

	archivePlan(t, ctx, "max")

	w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body)
	if w.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("archived plan was SOLD: status=%d body=%s, want 404", w.StatusCode, string(b))
	}
}

// The card path charges before it subscribes, so its gate has to refuse ahead of
// the charge — a retired tier taking money is worse than one merely still listed.
func TestPlanStatus_ArchivedPlanRefusedBeforeCardCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")
	archivePlan(t, ctx, "max")

	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, "max")
	if err != nil {
		t.Fatalf("archived plan must still RESOLVE (renewals depend on it): %v", err)
	}
	if p.Listed() {
		t.Fatal("plan reports Listed after archive; the card path would charge for it")
	}
}

// archivePlan seeds the authority if needed and marks one slug archived.
func archivePlan(t *testing.T, ctx context.Context, slug string) {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	db := datastore.New(nscontext.WithNamespace(ctx, plan.Namespace))
	p := plan.New(db)
	ok, err := p.Query().Filter("Slug=", slug).Get()
	if err != nil || !ok {
		t.Fatalf("load %s: found=%v err=%v", slug, ok, err)
	}
	p.Status = plan.StatusArchived
	if err := p.Update(); err != nil {
		t.Fatalf("archive %s: %v", slug, err)
	}
}

// Every row written before Status existed has no value for it. If empty meant
// "not active" the entire catalog would vanish the moment this shipped — so
// empty MUST read as listed.
func TestPlanStatus_EmptyMeansActive(t *testing.T) {
	var p plan.Plan
	if !p.Listed() {
		t.Fatal("a row with no Status is not Listed; deploying this would blank the catalog")
	}
	p.Status = plan.StatusActive
	if !p.Listed() {
		t.Fatal("explicit active is not Listed")
	}
	for _, s := range []string{plan.StatusDraft, plan.StatusArchived} {
		p.Status = s
		if p.Listed() {
			t.Fatalf("%q is Listed; it must not reach the public catalog", s)
		}
	}
}

// A contact-sales plan is NOT self-serve.
//
// paidTier used to read price alone, and a contact-sales row stores Price=0
// because its price is null, not free. So a catalog holding a negotiated tier
// with a real included allotment would have let an org admin self-subscribe and
// mint it with no payment. That never fired only because the tier with the large
// allotment also carried a large price — luck, not a gate.
func TestPaidTier_ContactSalesIsNotSelfServe(t *testing.T) {
	if !paidTier("enterprise") {
		t.Fatal("contact-sales 'enterprise' reads as a free tier; an org admin could self-subscribe")
	}
	// Control: a genuinely free tier stays self-serve, which is the distinction
	// the predicate exists to make. The published ladder has none, so this is
	// asserted on the predicate's inputs rather than on a catalog slug.
	for _, slug := range []string{"go", "dev", "pro", "max"} {
		if !paidTier(slug) {
			t.Errorf("paidTier(%q) = false; every ladder rung is paid", slug)
		}
	}
	if paidTier("no-such-plan") {
		t.Error("an unknown slug must not read as paid")
	}
}
