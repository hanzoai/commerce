package catalogentry

import (
	"strings"
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// lineFor is the line Book read for slug.
func lineFor(t *testing.T, lines []Line, slug string) Line {
	t.Helper()
	for _, l := range lines {
		if l.Slug == slug {
			return l
		}
	}
	t.Fatalf("%s has no line", slug)
	return Line{}
}

// The book is what a call is charged at: an admin's Price where one is set, else the
// sync's Cost times the entry's Markup; the Cost beside it; rung by rung, the open
// rung last; listed only while published and enabled. A row whose rates mix units
// prices nothing and is named, never guessed at.
func TestBookReadsPriceAndCostByRung(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	put := func(slug string, markup string, published bool, spec *ModelSpec, rates []Rate) {
		t.Helper()
		e := New(db)
		e.Slug, e.Category, e.Markup, e.Published, e.Spec, e.Rates = slug, CategoryModel, markup, published, spec, rates
		if err := e.Create(); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
	}
	put("resold", "1.2", true, &ModelSpec{Family: FamilyThirdParty, Enabled: true},
		[]Rate{
			{Key: RateIn, Unit: UnitMTok, Cost: "2", MaxContext: 200000},
			{Key: RateOut, Unit: UnitMTok, Cost: "10", MaxContext: 200000},
			{Key: RateIn, Unit: UnitMTok, Cost: "4"},
			{Key: RateOut, Unit: UnitMTok, Cost: "20", Price: "30"},
		})
	put("withdrawn", "", true, &ModelSpec{Enabled: false, Unavailable: WithdrawnUpstream},
		[]Rate{{Key: RateIn, Unit: UnitMTok, Price: "1"}, {Key: RateOut, Unit: UnitMTok, Price: "2"}})
	put("picture", "", false, nil, []Rate{{Unit: UnitImage, Price: "0.04"}})
	put("muddled", "", false, nil, []Rate{{Key: RateIn, Unit: UnitMTok, Price: "1"}, {Unit: UnitImage, Price: "0.04"}})

	lines, err := Book(db)
	if err == nil || !strings.Contains(err.Error(), "muddled") {
		t.Fatalf("a row that mixes units was not named: %v", err)
	}
	for _, l := range lines {
		if l.Slug == "muddled" {
			t.Fatal("a row that mixes units was read as a price")
		}
	}

	r := lineFor(t, lines, "resold")
	if r.Unit != UnitMTok || !r.Listed || r.Family != FamilyThirdParty || len(r.Rungs) != 2 {
		t.Fatalf("resold = %+v", r)
	}
	short, long := r.Rungs[0], r.Rungs[1]
	if short.Max != 200000 || short.Price.In != "2.4" || short.Price.Out != "12" || short.Cost.In != "2" || short.Cost.Out != "10" {
		t.Errorf("the short rung = %+v, want cost × 1.2 over a 2 / 10 cost", short)
	}
	if long.Max != 0 || long.Price.In != "4.8" || long.Price.Out != "30" || long.Cost.Out != "20" {
		t.Errorf("the open rung = %+v, want 4.8 and the admin's 30 over a 20 cost", long)
	}
	if w := lineFor(t, lines, "withdrawn"); w.Listed || w.Rungs[0].Price.In != "1" {
		t.Errorf("a withdrawn row = %+v, want priced and not listed", w)
	}
	if p := lineFor(t, lines, "picture"); p.Unit != UnitImage || p.Listed || p.Rungs[0].Price.In != "0.04" {
		t.Errorf("an image row = %+v, want 0.04 an image, unlisted", p)
	}
}

// The seeds are the book on a fresh store: every Enso SKU at the owner's price,
// listed in its family (the vision engines priced but unlisted), and every model
// the model API used to price in code at the rate production billed.
func TestTheSeedsAreTheBook(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)
	if _, err := SeedEnsoModels(db); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedModels(db); err != nil {
		t.Fatal(err)
	}
	lines, err := Book(db)
	if err != nil {
		t.Fatal(err)
	}
	for slug, want := range billedRetail {
		l := lineFor(t, lines, slug)
		p := l.Rungs[0].Price
		if p.In != want.in || p.Out != want.out || p.Read != want.read || p.Write != want.write || l.Family != FamilyEnso {
			t.Errorf("%s = %+v in family %q, want %+v in enso", slug, p, l.Family, want)
		}
		if internal := strings.HasPrefix(slug, "enso-vl"); l.Listed == internal {
			t.Errorf("%s listed %v", slug, l.Listed)
		}
	}
	for slug, want := range map[string][3]string{
		"kai": {UnitMTok, "0.021", "0"}, "hanzo/kai": {UnitMTok, "0.021", "0"},
		"typesafe/jev-1.13": {UnitMTok, "0.042", "0"}, "~typesafe/jev-latest": {UnitMTok, "0.042", "0"},
		"zenlm/zen6": {UnitMTok, "2.28", "7.26"}, "bge-m3": {UnitMTok, "0.02", "0"},
		"zen-scribe": {UnitSecond, "0.0001", ""}, "whisper": {UnitSecond, "0.0001", ""},
		"zen-voice-mini": {UnitMChar, "15", ""}, "kokoro": {UnitMChar, "15", ""},
		"stable-diffusion-3.5-large": {UnitImage, "0.08", ""}, "fal-ai/flux/schnell": {UnitImage, "0.05", ""},
		"wan2-2-t2v-a14b": {UnitVideo, "0.40", ""},
	} {
		l := lineFor(t, lines, slug)
		if p := l.Rungs[0].Price; l.Unit != want[0] || p.In != want[1] || p.Out != want[2] {
			t.Errorf("%s = %s %+v, want %s %s / %s", slug, l.Unit, p, want[0], want[1], want[2])
		}
	}
}

// A model row with a price is the only way a model is sold, so the seed names no
// model without one: a token row prices input and output, a unit row its unit.
func TestTheModelSeedPricesEveryRow(t *testing.T) {
	rows, err := ModelSeedRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		keys := map[string]bool{}
		for _, rate := range r.Rates {
			if rate.Price == "" {
				t.Errorf("%s: rate %q states no price", r.Slug, rate.Key)
			}
			keys[rate.Key] = true
		}
		if r.Rates[0].Unit == UnitMTok && (!keys[RateIn] || !keys[RateOut]) {
			t.Errorf("%s: a token row prices input and output", r.Slug)
		}
	}
}
