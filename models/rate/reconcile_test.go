package rate

import (
	"reflect"
	"testing"
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
