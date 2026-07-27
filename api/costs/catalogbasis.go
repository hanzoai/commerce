package costs

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/catalogentry"
)

// The synced catalog is the source of provider cost; costBasisTable is the
// fallback for models it does not cover. Two tables of what we pay upstream is
// the thing this file exists to end: the catalog is written by a sync from the
// provider, the table is written by hand, and when they disagree the hand-written
// one is the one that is wrong.
//
// This puts a datastore read behind cost-of-goods, so it is cached, and the
// cache IS the fail-open: a failed or empty read never replaces a good snapshot.
// A catalog outage degrades to the curated table (COGS a little stale) rather
// than to zero cost (COGS absent, margin reported as 100%). Overstating margin
// because a database blinked is the failure worth engineering against.
const (
	basisTTL     = 5 * time.Minute
	basisFailTTL = 30 * time.Second
	basisTimeout = 4 * time.Second
)

var (
	basisMu    sync.RWMutex
	basisSnap  map[string]costRate // last GOOD index; nil until a first success
	basisUntil time.Time           // zero = never attempted
	basisGroup singleflight.Group
	basisNow   = time.Now // test seam
)

// catalogRates returns the synced catalog indexed by the model id a ledger row
// carries. The returned map is read-only and shared; callers must not write it.
//
// It deliberately takes NO context. Two hazards die structurally:
//
//   - A caller's ctx is org-namespaced. Reading the platform "system" catalog
//     through it returns nothing, which presents as "every model unknown" — the
//     silent COGS-to-zero this file is written to prevent.
//   - A caller's ctx may already be most of the way through a vendor call. A
//     cancel landing mid-refresh would poison the snapshot for everyone.
func catalogRates() map[string]costRate {
	basisMu.RLock()
	snap, until := basisSnap, basisUntil
	basisMu.RUnlock()
	if !until.IsZero() && basisNow().Before(until) {
		return snap // may be nil: a cold-start failure is cached too, so a
		// broken store cannot turn every report into a query storm.
	}

	v, _, _ := basisGroup.Do("catalog", func() (any, error) {
		// Re-check: a concurrent flight may have refreshed while we queued.
		basisMu.RLock()
		snap, until := basisSnap, basisUntil
		basisMu.RUnlock()
		if !until.IsZero() && basisNow().Before(until) {
			return snap, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), basisTimeout)
		defer cancel()

		fresh, err := loadRates(catalogentry.SystemDB(ctx))

		basisMu.Lock()
		defer basisMu.Unlock()
		if err != nil || len(fresh) == 0 {
			// Keep the last good snapshot and retry sooner. An empty catalog is
			// treated as a failure on purpose: a sync that wrote nothing, or a
			// namespace that resolved wrong, is indistinguishable here from a
			// real outage, and neither should zero out cost.
			basisUntil = basisNow().Add(basisFailTTL)
			return basisSnap, nil
		}
		basisSnap = fresh
		basisUntil = basisNow().Add(basisTTL)
		return basisSnap, nil
	})

	rates, _ := v.(map[string]costRate)
	return rates
}

// loadRates reads every catalog entry and indexes the ones that price tokens.
func loadRates(db *datastore.Datastore) (map[string]costRate, error) {
	entries := make([]*catalogentry.CatalogEntry, 0, 256)
	if _, err := catalogentry.Query(db).GetAll(&entries); err != nil {
		return nil, err
	}
	return indexRates(entries), nil
}

// indexRates is the join between a metered model id and a catalog slug, and it
// is pure so that join can be tested without a datastore.
//
// Published, Spec.Enabled and availability are all IGNORED. We paid for those
// tokens whether or not the row is still listed or callable, so filtering by
// visibility here would delete real cost from the books.
//
// Each entry is registered under its slug, its upstream id when that differs,
// and the tail after the last "/" — a ledger's model is whatever the metering
// caller sent, which is often the unqualified id. A tail claimed by two entries
// with DIFFERENT rates is dropped rather than resolved arbitrarily: an ambiguous
// join falls through to the curated table, which is at least deliberate.
func indexRates(entries []*catalogentry.CatalogEntry) map[string]costRate {
	out := make(map[string]costRate, len(entries)*2)
	ambiguous := make(map[string]bool)

	put := func(key string, r costRate) {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" || ambiguous[k] {
			return
		}
		if prev, seen := out[k]; seen && prev != r {
			delete(out, k)
			ambiguous[k] = true
			return
		}
		out[k] = r
	}

	for _, e := range entries {
		in, outc, ok := catalogentry.TokenCostCents(e)
		if !ok {
			continue
		}
		r := costRate{InputCentsPerMTok: in, OutputCentsPerMTok: outc}
		put(e.Slug, r)
		if e.Spec != nil && e.Spec.Upstream != "" {
			put(e.Spec.Upstream, r)
		}
		if i := strings.LastIndex(e.Slug, "/"); i >= 0 && i+1 < len(e.Slug) {
			put(e.Slug[i+1:], r)
		}
	}
	return out
}

// basisRate resolves a model's cost: the synced catalog by exact id first, then
// the curated table by longest prefix. The bool reports whether ANY source knew
// it, so unknown tokens keep being counted honestly rather than priced at zero.
func basisRate(catalog map[string]costRate, model string) (costRate, bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return costRate{}, false
	}
	if r, ok := catalog[m]; ok {
		return r, true
	}
	return lookupCostRate(m)
}
