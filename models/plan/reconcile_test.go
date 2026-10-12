package plan

import (
	"reflect"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

func n(i int) *int { return &i }

// THE INVARIANT copyInto's own comment states: every field it copies is one
// planEqual compares. This asserts it over the STRUCT rather than over a list
// somebody has to remember to extend — which is the only version that survives
// the next field. A field copied but not compared fails silently in the worst
// direction: a plan whose ONLY change is that field reads as already published,
// the seed skips it, and the catalog and the served row disagree with no log.
func TestEveryLimitCopiedIsAlsoCompared(t *testing.T) {
	tp := reflect.TypeOf(Limits{})
	ptrInt := reflect.TypeOf((*int)(nil))
	checked := 0
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.Type != ptrInt {
			continue
		}
		a, b := &Limits{}, &Limits{}
		reflect.ValueOf(a).Elem().Field(i).Set(reflect.ValueOf(n(1)))
		reflect.ValueOf(b).Elem().Field(i).Set(reflect.ValueOf(n(2)))
		if limitsEqual(a, b) {
			t.Errorf("limitsEqual ignores Limits.%s, so a plan whose only change is that "+
				"field reads as already published: the seed writes nothing, the catalog "+
				"and the served row disagree, and no log says so", f.Name)
		}
		checked++
	}
	if checked < 12 {
		t.Fatalf("only %d fields examined — this guard is watching almost nothing", checked)
	}
}

// A plan whose ONLY change is a capacity must reconcile. This is the production
// case in miniature: same slug, same price, a new roster.
func TestACapacityChangeAloneIsReconciled(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	before := []*Plan{{
		Slug: "free", Category: "personal", Name: "Free", Price: 0, Currency: currency.USD,
		Limits: &Limits{Agents: n(1)},
	}}
	if _, _, err := Seed(db, before); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The catalog now says the free rung includes no bots. Nothing else moved.
	after := []*Plan{{
		Slug: "free", Category: "personal", Name: "Free", Price: 0, Currency: currency.USD,
		Limits: &Limits{Agents: n(1), Bots: n(0)},
	}}
	_, corrected, err := Seed(db, after)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if corrected == 0 {
		t.Fatal("the catalog published a new capacity and the seed wrote nothing — a plan " +
			"that changed only its roster stays at its old row")
	}

	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "free").Get(); !ok {
		t.Fatal("free is missing")
	}
	if p.Limits == nil || p.Limits.Bots == nil || *p.Limits.Bots != 0 {
		t.Fatalf("stored limits = %+v, want the catalog's bots=0", p.Limits)
	}
}

// A RETIRED PLAN COMES BACK WHEN THE CATALOG PUBLISHES IT AGAIN. The catalog
// already decides retirement; this is the same sentence read the other way.
func TestARepublishedPlanIsSoldAgain(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	dev := &Plan{Slug: "dev", Category: "personal", Name: "Dev", Price: 1900, Currency: currency.USD}
	if _, _, err := Seed(db, []*Plan{dev}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The catalog stops publishing it — the seed archives it, which is right.
	if _, _, err := Seed(db, []*Plan{{Slug: "pro", Category: "personal", Name: "Pro", Price: 4900, Currency: currency.USD}}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "dev").Get(); !ok {
		t.Fatal("dev was deleted rather than archived")
	}
	if p.Listed() {
		t.Fatal("dev is still listed after the catalog dropped it")
	}

	// It returns to the catalog.
	if _, _, err := Seed(db, []*Plan{dev}); err != nil {
		t.Fatalf("republish: %v", err)
	}
	q := New(db)
	if ok, _ := q.Query().Filter("Slug=", "dev").Get(); !ok {
		t.Fatal("dev is missing")
	}
	if !q.Listed() {
		t.Fatal("the catalog publishes dev and it is still not sold — a retired tier " +
			"can never come back, and the seed reports success while the public " +
			"catalog keeps saying it does not exist")
	}
}

// An admin's decision still wins. Reviving must not undo a human who deliberately
// unlisted a plan the catalog still publishes.
func TestAnAdminUnlistingSurvivesTheSeed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	row := &Plan{Slug: "pro", Category: "personal", Name: "Pro", Price: 4900, Currency: currency.USD}
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	p.Status = StatusDraft
	p.AdminEdited = true
	if err := p.Update(); err != nil {
		t.Fatalf("admin edit: %v", err)
	}

	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	q := New(db)
	if ok, _ := q.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	if q.Listed() {
		t.Error("the seed put back a plan an admin deliberately unlisted — the package " +
			"is the source, an admin edit is an override, and reviving must not reverse that")
	}
}

// A DRAFT ROW IS BORN UNLISTED AND STAYS SO. A catalog price is chargeable the
// moment a deploy carries it, so the catalog can publish a rung as a draft: the
// seed creates it, no purchase entrypoint sells it (Listed), and a second boot
// leaves it exactly where the catalog put it.
func TestADraftIsCreatedUnlistedAndStaysUnlisted(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	draft := &Plan{Slug: "max-100x", Category: "personal", Name: "Max 100x", Price: 100000, Currency: currency.USD, Status: StatusDraft}
	for boot := 1; boot <= 2; boot++ {
		if _, _, err := Seed(db, []*Plan{draft}); err != nil {
			t.Fatalf("boot %d: %v", boot, err)
		}
		p := New(db)
		if ok, _ := p.Query().Filter("Slug=", "max-100x").Get(); !ok {
			t.Fatalf("boot %d: the draft was not created", boot)
		}
		if p.Listed() {
			t.Fatalf("boot %d: a draft row is on sale", boot)
		}
	}
}

// THE CATALOG MAKES A DRAFT SELLABLE, AND CAN TAKE A ROW BACK TO DRAFT. Active and
// draft are one field read in both directions, so activating a rung is one edit
// to the catalog and withdrawing it is the same edit reversed.
func TestTheCatalogActivatesAndWithdrawsADraft(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	row := &Plan{Slug: "max-50x", Category: "personal", Name: "Max 50x", Price: 50000, Currency: currency.USD, Status: StatusDraft}
	listed := func() bool {
		p := New(db)
		if ok, _ := p.Query().Filter("Slug=", "max-50x").Get(); !ok {
			t.Fatal("max-50x is missing")
		}
		return p.Listed()
	}
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatal(err)
	}
	if listed() {
		t.Fatal("the draft is on sale")
	}

	row.Status = ""
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatal(err)
	}
	if !listed() {
		t.Fatal("the catalog made the row active and it is still not on sale")
	}

	row.Status = StatusDraft
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatal(err)
	}
	if listed() {
		t.Fatal("the catalog withdrew the row and it is still on sale")
	}
}

// An admin's listing still beats the catalog's draft: the package is the source,
// a human edit is an override.
func TestAnAdminListingSurvivesACatalogDraft(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	row := &Plan{Slug: "max-50x", Category: "personal", Name: "Max 50x", Price: 50000, Currency: currency.USD, Status: StatusDraft}
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatal(err)
	}
	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "max-50x").Get(); !ok {
		t.Fatal("max-50x is missing")
	}
	p.Status = StatusActive
	p.AdminEdited = true
	if err := p.Update(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Seed(db, []*Plan{row}); err != nil {
		t.Fatal(err)
	}
	q := New(db)
	if ok, _ := q.Query().Filter("Slug=", "max-50x").Get(); !ok || !q.Listed() {
		t.Fatal("the seed reversed an admin's listing")
	}
}
