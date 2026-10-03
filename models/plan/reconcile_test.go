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
