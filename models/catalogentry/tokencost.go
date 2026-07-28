package catalogentry

// TokenCostCents reports what one million input tokens and one million output
// tokens of this entry cost US upstream, in cents. ok is false when the entry
// prices no per-token component at all.
//
// This is a REPORTING unit, not a billing one. Cents-per-Mtok is not integral —
// an upstream at $0.000000003625 per token is 0.3625 c/Mtok — so it cannot be a
// currency.Cents, and float64 is only admissible because the caller rounds to
// whole cents over an aggregate of many rows. To BILL a rate, parse Rate.Cost as
// money and multiply there; never round-trip through this.
//
// The rules below are each a refusal to guess:
//
//   - Unit must be UnitMTok. RatesOf synthesizes month/hour rates for infra rows,
//     and reading one of those as a per-token figure produces a number that is
//     wrong by orders of magnitude rather than merely imprecise.
//
//   - Only the MaxContext == 0 rate is read. Context rungs are data with no
//     selector in this repo, and a token ledger carries no context length to pick
//     one with. An entry that prices ONLY rungs reports ok=false and lets the
//     caller fall back, which is honest; guessing a rung is not.
//
//   - An absent in/out component is zero, not unknown. The upstream decoder omits
//     a component the provider prices at zero, so absence there means free.
//
//   - An entry with no parseable mtok cost at all reports ok=false. A sync can
//     wipe rates when an upstream stops publishing them, and an absence must
//     never be published as a zero cost — that reads downstream as 100% margin.
func TokenCostCents(e *CatalogEntry) (in, out float64, ok bool) {
	if e == nil {
		return 0, 0, false
	}
	// RatesOf may return the entry's live backing slice; read only.
	for _, r := range RatesOf(e) {
		if r.Unit != UnitMTok || r.MaxContext != 0 {
			continue
		}
		if r.Key != RateIn && r.Key != RateOut {
			continue // cacheRead/cacheWrite: the ledger meters no cache split
		}
		d, parsed := dec(r.Cost)
		if !parsed {
			continue
		}
		// USD -> cents is an exact decimal shift; the float conversion happens
		// once here, at the snapshot boundary, not per ledger row.
		cents := d.Shift(2).InexactFloat64()
		switch r.Key {
		case RateIn:
			in, ok = cents, true
		case RateOut:
			out, ok = cents, true
		}
	}
	return in, out, ok
}
