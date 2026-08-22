package rate

import (
	"reflect"
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// THE SAME INVARIANT THE PLAN SEED HAS, asserted the same way: every field
// copyInto writes is one equal compares.
//
// It is not a style rule. equal decides whether the seed writes at all, so a
// field copied but not compared can never be corrected — the stored row and the
// published catalog disagree, the reseed reports nothing to do, and no log says
// so. The plan seed shipped exactly that bug to production with four fields; this
// walks the struct so a rate cannot repeat it.
//
// Two fields are deliberately outside it:
//
//	AdminEdited  never copied — it is the flag that makes the seed SKIP a row,
//	             so comparing it would be asking whether to overwrite a human.
//	Slug         never typed, derived by Bind from Product+Meter. Comparing the
//	             parts covers it, and the lookup that found the row matched it.
func TestEveryFieldCopiedIsAlsoCompared(t *testing.T) {
	skip := map[string]string{
		"AdminEdited": "never copied — it is what makes the seed skip a row",
		"Slug":        "derived by Bind from Product+Meter, which are compared",
	}

	tp := reflect.TypeOf(Rate{})
	checked := 0
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.PkgPath != "" || f.Anonymous {
			continue // unexported, or the embedded mixin
		}
		if _, ok := skip[f.Name]; ok {
			continue
		}

		a, b := &Rate{}, &Rate{}
		av := reflect.ValueOf(a).Elem().Field(i)
		bv := reflect.ValueOf(b).Elem().Field(i)
		switch f.Type.Kind() {
		case reflect.String:
			av.SetString("one")
			bv.SetString("two")
		case reflect.Int, reflect.Int64:
			av.SetInt(1)
			bv.SetInt(2)
		case reflect.Bool:
			av.SetBool(false)
			bv.SetBool(true)
		default:
			t.Fatalf("Rate.%s is a %s and this guard does not know how to vary it — "+
				"teach it rather than letting the field go unchecked", f.Name, f.Type.Kind())
		}

		if equal(a, b) {
			t.Errorf("equal() ignores Rate.%s, so a row whose only change is that field "+
				"can never be corrected: the seed writes nothing, the authority and the "+
				"catalog disagree, and nothing reports it", f.Name)
		}
		checked++
	}
	if checked < 7 {
		t.Fatalf("only %d fields examined — this guard is watching almost nothing", checked)
	}
}

// A row that carries the right slug and nothing else must be repaired. This is
// the partial-row case: identity present, parts empty, which is what a lookup by
// slug finds and what equal used to call already-correct.
func TestAPartialRowIsRepaired(t *testing.T) {
	stored := &Rate{Slug: "ai/zen-coder", Label: "Zen Coder", Unit: "token",
		Rate: 250, Currency: "USD", Source: "catalog"}
	published := &Rate{Product: "ai", Meter: "zen-coder", Label: "Zen Coder",
		Unit: "token", Rate: 250, Currency: "USD", Source: "catalog"}

	if equal(stored, published) {
		t.Fatal("a row with an empty Product and Meter reads as already published, so " +
			"the seed leaves it half-written forever — and an editor that groups by " +
			"product cannot see it at all")
	}

	stored.Take(published)
	if stored.Product != "ai" || stored.Meter != "zen-coder" {
		t.Fatalf("Take left the parts as %q/%q", stored.Product, stored.Meter)
	}
	if stored.Slug != "ai/zen-coder" {
		t.Fatalf("Bind produced %q, want ai/zen-coder", stored.Slug)
	}
	if !equal(stored, published) {
		t.Error("a repaired row still differs from what was published, so the next " +
			"reseed corrects it again and the row never converges")
	}
}

// A PRICE THAT IS NOT SOLD MUST NOT BE CHARGED, and the predicate that decides
// admits only what it recognises.
//
// It read `!archived` before, so every other value — "draft", a typo, a state
// added later and not taught here — meant SOLD. Draft is the one that mattered:
// the word withholds a plan on the model next door, so an operator who learned
// it there would have published a half-written price by saving it.
func TestOnlyARecognisedStatusIsSold(t *testing.T) {
	for status, sold := range map[string]bool{
		"":         true, // unset is active
		"active":   true,
		"ACTIVE":   true, // the comparison is case-insensitive
		"archived": false,
		"draft":    false,
		"Draft":    false,
		"pending":  false, // never defined, therefore not sold
		"actve":    false, // a typo must withhold a price, not publish one
	} {
		if got := (&Rate{Status: status}).Listed(); got != sold {
			t.Errorf("status %q listed=%v, want %v — an unrecognised status must withhold "+
				"the price, because a caller that reads none falls back to the floor it "+
				"charged yesterday while one that reads a wrong price bills a customer",
				status, got, sold)
		}
	}
}

// The seed's ERROR paths and its protections, which the happy path never reaches.
func TestSeedSkipsWhatItMustNotWrite(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	// A row with no identity cannot be keyed, so it is skipped rather than
	// written under the empty slug — where it would then be the answer for every
	// unnamed lookup.
	created, corrected, err := Seed(db, []*Rate{
		nil,
		{Product: "", Meter: ""},
		{Product: "storage", Meter: ""},
		{Product: "", Meter: "screen"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 0 || corrected != 0 {
		t.Errorf("seed wrote created=%d corrected=%d for rows with no identity", created, corrected)
	}

	// An admin's price outranks the published one, and the seed must not touch it.
	row := New(db)
	row.Product, row.Meter, row.Rate, row.AdminEdited = "risk", "screen", 250_000, true
	row.Bind()
	if err := row.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := Seed(db, []*Rate{{Product: "risk", Meter: "screen", Rate: 100_000}}); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "risk/screen").Get(); !ok {
		t.Fatal("risk/screen vanished")
	}
	if got.Rate != 250_000 {
		t.Errorf("the seed reverted an operator's price to %d — AdminEdited is the whole "+
			"contract with the importer, and without it every edit is temporary", got.Rate)
	}
}

// Seeding is idempotent AND it corrects: the second run writes nothing, and a
// changed catalog moves exactly the rows that changed.
func TestSeedWritesOnlyWhatMoved(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	rows := []*Rate{
		{Product: "storage", Meter: "block-gb-month", Unit: "GB-month", Rate: 80_000_000, Currency: "USD"},
		{Product: "risk", Meter: "screen", Unit: "screen", Rate: 100_000, Currency: "USD"},
	}
	created, corrected, err := Seed(db, rows)
	if err != nil || created != 2 || corrected != 0 {
		t.Fatalf("first seed created=%d corrected=%d err=%v, want 2/0", created, corrected, err)
	}

	created, corrected, err = Seed(db, rows)
	if err != nil || created != 0 || corrected != 0 {
		t.Fatalf("re-seed created=%d corrected=%d err=%v — a document that changed nothing "+
			"must write nothing, or every boot fills the audit trail with no-ops",
			created, corrected, err)
	}

	// One price moves. Exactly one row is corrected.
	rows[1].Rate = 120_000
	created, corrected, err = Seed(db, rows)
	if err != nil || created != 0 || corrected != 1 {
		t.Fatalf("moved seed created=%d corrected=%d err=%v, want 0/1", created, corrected, err)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "risk/screen").Get(); !ok || got.Rate != 120_000 {
		t.Errorf("risk/screen = %d, want the published 120000", got.Rate)
	}
}

// Take is the one definition of what a write may set, so what it LEAVES is as
// load-bearing as what it copies.
func TestTakeLeavesTheBookkeepingAlone(t *testing.T) {
	dst := &Rate{Product: "old", Meter: "thing", AdminEdited: true, Status: "archived"}
	dst.Bind()

	// A source that states no status must not unlist — or relist — the row.
	dst.Take(&Rate{Product: "storage", Meter: "block-gb-month", Rate: 1})
	if dst.Status != "archived" {
		t.Errorf("status = %q; a document that says nothing about status must not change it", dst.Status)
	}
	if !dst.AdminEdited {
		t.Error("Take cleared AdminEdited — whether a write is a person's decision is a " +
			"fact about the request, not about the values it carried")
	}
	if dst.Slug != "storage/block-gb-month" {
		t.Errorf("slug = %q; Take rebinds from the parts it just took", dst.Slug)
	}

	// A source that DOES state one moves it.
	dst.Take(&Rate{Product: "storage", Meter: "block-gb-month", Status: "active"})
	if dst.Status != "active" {
		t.Errorf("status = %q, want active", dst.Status)
	}
}
