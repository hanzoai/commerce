package catalogentry

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hanzoai/commerce/datastore"
)

// THE CATALOG IS WHAT EVERY MODEL CALL IS CHARGED AT.
//
// Book is the catalog as a host's billing reads it: one line a model, its price
// (what the customer pays: an admin's Price, else the sync's Cost times the entry's
// Markup — RetailPrice) and its cost (what we pay: the sync's Cost), by context rung.
// The model API (hanzoai/ai controllers/tariff.go) quotes every charge from these
// lines, versions each by its content, and records the version and the rate on every
// usage event; it holds no price of its own. A model with no line is not sold — there
// is no default — and a free model is a line priced at zero.

// Line is one model's row of the pricing catalog.
type Line struct {
	// Slug is the model id a call names.
	Slug string
	// Family is the family the row belongs to (enso, zen, third-party, or none).
	Family string
	// Listed says the model is on sale: published and enabled. An unlisted row still
	// prices a call that names it, so a past or internal model is never unpriced.
	Listed bool
	// Unit is what one billed quantity is (UnitMTok and the rest).
	Unit  string
	Rungs []Rung
}

// Rung is one context rung's price and cost, USD per Unit, exact decimals; "" is
// unstated.
type Rung struct {
	// Max is the longest prompt the rung prices; 0 is every prompt.
	Max   int
	Price Quad
	Cost  Quad
}

// Quad is a rung's four token classes; a per-unit rate is In alone.
type Quad struct{ In, Out, Read, Write string }

// Book reads every model row of the catalog in db (which MUST be the "system"
// namespace, SystemDB) as lines. A row whose rates mix units prices nothing it can
// say, so it is left out and named in the error beside the lines that read.
func Book(db *datastore.Datastore) ([]Line, error) {
	var out []Line
	var bad []string
	for _, cat := range modelCategories {
		var entries []*CatalogEntry
		if _, err := Query(db).Filter("Category=", cat).GetAll(&entries); err != nil {
			return nil, fmt.Errorf("catalog: list %s models: %w", cat, err)
		}
		for _, e := range entries {
			if e == nil || e.Slug == "" {
				continue
			}
			l, ok := lineOf(e)
			if !ok {
				bad = append(bad, e.Slug)
				continue
			}
			out = append(out, l)
		}
	}
	slices.SortFunc(out, func(a, b Line) int { return strings.Compare(a.Slug, b.Slug) })
	if len(bad) > 0 {
		return out, fmt.Errorf("catalog: %s mix units and price nothing", strings.Join(bad, ", "))
	}
	return out, nil
}

// lineOf is one entry's line, false for an entry whose rates mix units.
func lineOf(e *CatalogEntry) (Line, bool) {
	l := Line{Slug: strings.ToLower(e.Slug), Listed: e.Published && (e.Spec == nil || e.Spec.Enabled)}
	if e.Spec != nil {
		l.Family = e.Spec.Family
	}
	rungs := map[int]*Rung{}
	for _, r := range RatesOf(e) {
		if l.Unit == "" {
			l.Unit = r.Unit
		}
		if r.Unit != l.Unit {
			return Line{}, false
		}
		g, ok := rungs[r.MaxContext]
		if !ok {
			g = &Rung{Max: r.MaxContext}
			rungs[r.MaxContext] = g
		}
		price, cost := r.RetailPrice(e.Markup), r.Cost
		switch r.Key {
		case RateIn, "":
			g.Price.In, g.Cost.In = price, cost
		case RateOut:
			g.Price.Out, g.Cost.Out = price, cost
		case RateCacheRead:
			g.Price.Read, g.Cost.Read = price, cost
		case RateCacheWrite:
			g.Price.Write, g.Cost.Write = price, cost
		}
	}
	for _, g := range rungs {
		l.Rungs = append(l.Rungs, *g)
	}
	slices.SortFunc(l.Rungs, func(a, b Rung) int { return open(a.Max) - open(b.Max) })
	return l, true
}

// open orders rungs: the open rung (Max 0) after every bounded one.
func open(max int) int {
	if max == 0 {
		return int(^uint(0) >> 1)
	}
	return max
}
