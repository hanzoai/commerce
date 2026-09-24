package catalogentry

import (
	"errors"
	"time"

	"github.com/hanzoai/commerce/datastore"
)

// Refresh is a scheduled PULL of one upstream's catalog. UpsertModels is the
// write seam and stays exactly that — this adds the two things a run needs that
// a push of already-decided rows does not:
//
//  1. A WITHDRAWAL SWEEP. Upsert only ever sees the models an upstream still
//     publishes, so on its own a model that disappears stays listed and routable
//     forever. Refresh compares the run against what the upstream owned before
//     it and withdraws the difference.
//
//  2. A FAIL-SAFE. A sync that cannot read its upstream must change NOTHING.
//     Zero models is indistinguishable from an outage, and believing it would
//     withdraw the entire catalog in one run.
//
// A withdrawn model is MARKED, never deleted: usage rows, invoices and old
// conversations reference it by slug, and deleting it turns every one of those
// into a dangling id. It stays published and priced — an entry with an honest
// reason it cannot be called is more useful than a row that vanished.

// ErrEmptyUpstream is a REFUSAL, not a result. See the fail-safe above.
var ErrEmptyUpstream = errors.New("refusing to sync an empty upstream catalog")

// WithdrawnUpstream is the reason stamped on a model the upstream stopped
// publishing. It is the entry's ModelSpec.Unavailable — visible to every caller,
// because a model you cannot call for a stated reason is an honest listing and a
// model that silently disappeared is not.
const WithdrawnUpstream = "withdrawn by the upstream provider"

// RefreshResult is what one run did, in the terms a review needs: what the
// upstream published, what moved, and what stopped being callable.
type RefreshResult struct {
	Serves    string   `json:"serves"`
	Upstream  int      `json:"upstream"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Withdrawn []string `json:"withdrawn,omitempty"`
	Restored  []string `json:"restored,omitempty"`
	SyncedAt  string   `json:"syncedAt"`
}

// Refresh lands rows for the upstream named by serves and withdraws whatever
// that upstream no longer publishes.
//
// The sweep is SCOPED BY SERVES, so one upstream going dark can never withdraw
// another family's models: an OpenRouter outage cannot touch Enso or Zen.
//
// It is idempotent in the sense that matters — the same upstream payload yields
// the same catalog state, and a second run withdraws and restores nothing.
//
// db MUST be namespaced to the catalog-owning "system" org.
func Refresh(db *datastore.Datastore, serves string, rows []ModelRow) (RefreshResult, error) {
	if len(rows) == 0 {
		return RefreshResult{}, ErrEmptyUpstream
	}
	if serves == "" {
		return RefreshResult{}, errors.New("refresh: serves is required (it scopes the withdrawal sweep)")
	}

	res := RefreshResult{
		Serves:   serves,
		Upstream: len(rows),
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Everything this upstream owned before the run. Read FIRST, so a row the run
	// creates is not mistaken for one it failed to mention.
	owned, err := servedBy(db, serves)
	if err != nil {
		return RefreshResult{}, err
	}

	stats, err := UpsertModels(db, rows)
	if err != nil {
		return RefreshResult{}, err
	}
	res.Created, res.Updated = stats.Created, stats.Updated

	published := make(map[string]bool, len(rows))
	for _, r := range rows {
		published[r.Slug] = true
	}

	// Re-read: the upsert wrote through its own handles, so the pre-run copies
	// are stale, and a model this run RESTORED must be seen in its new state.
	after, err := servedBy(db, serves)
	if err != nil {
		return RefreshResult{}, err
	}
	for _, e := range after {
		if e.Spec == nil {
			continue
		}
		switch {
		case owned[e.Slug] == nil:
			// Born in this run. Freeze its retail price now — see freezePrice.
			if !freezePrice(e) {
				continue
			}

		case published[e.Slug] && !e.Spec.Enabled && e.Spec.Unavailable == WithdrawnUpstream:
			// The upstream publishes it again. Only a model WE withdrew is
			// restored — an operator who disabled one by hand keeps that
			// decision, because it was never the upstream's to reverse.
			e.Spec.Enabled, e.Spec.Unavailable = true, ""
			res.Restored = append(res.Restored, e.Slug)

		case !published[e.Slug] && e.Spec.Enabled:
			e.Spec.Enabled, e.Spec.Unavailable = false, WithdrawnUpstream
			res.Withdrawn = append(res.Withdrawn, e.Slug)

		default:
			continue
		}
		if err := e.Update(); err != nil {
			return RefreshResult{}, err
		}
	}
	return res, nil
}

// freezePrice writes each new row's retail price out EXPLICITLY, as the number
// it is at birth, and reports whether it wrote anything.
//
// Rate.RetailPrice falls back to cost × markup when no price is stored, which is
// what spares a 341-row long tail from being hand-priced. But a price that is
// COMPUTED FROM A LIVE COST moves whenever the upstream moves: raise their cost
// and every customer's bill rises with it, decided by nobody. That is the one
// thing a sync must never cause.
//
// Freezing at creation gets both. The row arrives priced at cost × markup, so
// nothing needs hand-pricing and the day this lands changes no bill; and from
// then on the price is a stored number that only a human changes, so the next
// upstream move surfaces as a MARGIN THAT MOVED in admin.hanzo.ai rather than as
// a silent re-pricing. Creation is the only moment this is legitimate: the row
// is new, so there is no customer relying on a price to disturb.
func freezePrice(e *CatalogEntry) bool {
	wrote := false
	for i := range e.Rates {
		if e.Rates[i].Price != "" {
			continue
		}
		if price := e.Rates[i].RetailPrice(e.Markup); price != "" {
			e.Rates[i].Price = price
			wrote = true
		}
	}
	return wrote
}

// servedBy returns every catalog entry the named upstream is responsible for,
// bound to the store so it can be written back. A bulk read yields values, not
// bound entities; without the key an Update has no identity to write to.
func servedBy(db *datastore.Datastore, serves string) (map[string]*CatalogEntry, error) {
	entries := make([]*CatalogEntry, 0, 512)
	keys, err := Query(db).GetAll(&entries)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*CatalogEntry, len(entries))
	for i, e := range entries {
		if e.Spec == nil || e.Spec.Serves != serves {
			continue
		}
		e.Init(db)
		if i < len(keys) {
			if err := e.SetKey(keys[i]); err != nil {
				return nil, err
			}
		}
		out[e.Slug] = e
	}
	return out, nil
}
