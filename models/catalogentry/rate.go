package catalogentry

import (
	"math"

	"github.com/shopspring/decimal"

	"github.com/hanzoai/commerce/models/types/currency"
)

// Rate is ONE metered component of an entry's economics, in ONE unit.
//
// A price is a VECTOR, not a scalar: a model charges separately for input,
// output and cached reads, and may charge a different amount above a context
// rung. A VM is the degenerate one-element case (one rate, unit "month"). One
// shape expresses both, so there is exactly one way to read a price.
//
// Cost is what WE pay upstream — written only by a sync, never by hand.
// Price is what the CUSTOMER pays — set in admin.hanzo.ai, never by a sync.
// Margin is DERIVED from the two and displayed; it is never stored as a
// separate truth. When Price is empty the retail price is Cost x the entry's
// Markup, so a long tail of synced models needs no per-row hand-pricing.
//
// Amounts are exact decimal strings in USD per Unit. They are NOT cents:
// currency.Cents is an int64 and cannot represent $0.000015 per token, and a
// float would lose the upstream's published precision.
type Rate struct {
	Key        string `json:"key,omitempty"`        // "" (single) | in | out | cacheRead | cacheWrite
	Unit       string `json:"unit"`                 // mtok | month | hour | image | call | second
	MaxContext int    `json:"maxContext,omitempty"` // rung ceiling this rate applies up to; 0 = all
	Cost       string `json:"cost,omitempty"`       // upstream unit cost, USD — SYNCED
	Price      string `json:"price,omitempty"`      // retail unit price, USD — SET IN ADMIN
}

// Rate keys. A single-component price (a VM, a plan) leaves Key empty.
const (
	RateIn         = "in"
	RateOut        = "out"
	RateCacheRead  = "cacheRead"
	RateCacheWrite = "cacheWrite"
)

// Rate units.
const (
	UnitMTok   = "mtok" // per million tokens
	UnitMonth  = "month"
	UnitHour   = "hour"
	UnitImage  = "image"
	UnitCall   = "call"
	UnitSecond = "second"
)

// DefaultMarkup is the retail multiple applied to a synced upstream cost when
// an entry sets no Markup of its own. It matches the 1.20 spread the OpenRouter
// resale already ran at, so making markup per-entry changes no customer's price
// on the day it lands.
const DefaultMarkup = "1.20"

// same reports whether two rates address the same metered component — the rate
// IDENTITY, which is (key, unit, context rung). A sync matches on it to refresh
// a cost without disturbing the price a human set next to it.
func (r Rate) same(o Rate) bool {
	return r.Key == o.Key && r.Unit == o.Unit && r.MaxContext == o.MaxContext
}

// dec parses an exact decimal amount. An empty or unparseable amount is zero
// and reports false, so a missing number never reads as a real zero price.
func dec(s string) (decimal.Decimal, bool) {
	if s == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

// markupOf returns the entry's retail multiple, defaulting to DefaultMarkup.
// A markup below 1 is honored, not clamped: a loss-leader is a real business
// choice. It is visible as a negative margin in the admin projection rather
// than being made unrepresentable.
func markupOf(markup string) decimal.Decimal {
	if d, ok := dec(markup); ok {
		return d
	}
	d, _ := decimal.NewFromString(DefaultMarkup)
	return d
}

// RetailPrice returns what the customer pays for this rate: the explicit Price
// when an admin set one, else Cost x markup. Empty when neither is known — an
// unknown price is an absent field, never a fabricated 0.
func (r Rate) RetailPrice(markup string) string {
	if r.Price != "" {
		return r.Price
	}
	cost, ok := dec(r.Cost)
	if !ok {
		return ""
	}
	return cost.Mul(markupOf(markup)).Round(8).String()
}

// RateMarginPct returns the gross margin of a rate's retail price over its
// upstream cost, as a percentage rounded to two decimals: (price-cost)/price.
// nil when either side is unknown, so an uncomputable margin is an absent
// field rather than a misleading 0 or 100.
func RateMarginPct(r Rate, markup string) *float64 {
	price, okP := dec(r.RetailPrice(markup))
	cost, okC := dec(r.Cost)
	if !okP || !okC || price.IsZero() {
		return nil
	}
	pct, _ := price.Sub(cost).Div(price).Mul(decimal.NewFromInt(100)).Float64()
	pct = math.Round(pct*100) / 100
	return &pct
}

// scalarRate synthesizes the degenerate one-element rate vector from an entry's
// legacy scalar price, so EVERY entry projects a `rates` array and a reader has
// exactly one shape to understand. The unit follows the entry's category: GPU
// tiers bill hourly, everything else monthly. Returns nil when the entry
// carries no scalar price at all.
func scalarRate(e *CatalogEntry) []Rate {
	if e.PriceCents == 0 && e.CostCents == 0 {
		return nil
	}
	unit := UnitMonth
	if e.Category == "gpu" {
		unit = UnitHour
	}
	return []Rate{{
		Unit:  unit,
		Cost:  centsAmount(e.CostCents),
		Price: centsAmount(e.PriceCents),
	}}
}

// centsAmount renders cents as an exact USD decimal string ("" for zero, so an
// unset cost stays absent instead of reading as free).
func centsAmount(c currency.Cents) string {
	if c == 0 {
		return ""
	}
	return decimal.New(int64(c), -2).String()
}

// RatesOf returns an entry's price vector: the stored Rates when it has them,
// else the degenerate vector synthesized from its scalar PriceCents/CostCents.
// This is the ONE read path for a price — every projection goes through it, so
// a model and a VM are read the same way.
func RatesOf(e *CatalogEntry) []Rate {
	if len(e.Rates) > 0 {
		return e.Rates
	}
	return scalarRate(e)
}
