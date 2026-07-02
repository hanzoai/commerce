package seed

import (
	"context"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
)

// agencyCatalogOrg is the org (== tenant) whose per-org catalog backs the Hanzo
// Agency onboarding checkout. The agency BFF authenticates as this org and the
// checkout prices every plan from THIS store's listings — the sole server-side
// price authority.
const agencyCatalogOrg = "hanzo"

// agencyPlan is one plan the agency sells, priced in cents.
type agencyPlan struct {
	slug  string
	name  string
	price int64
}

// agencyPlans is the authoritative price list for the agency onboarding plans,
// mirroring the public pricing page. It exists so the catalog is ALWAYS present
// for the money path: the per-org store is the checkout price authority, and a
// missing catalog would fail every agency checkout closed.
var agencyPlans = []agencyPlan{
	{"agency", "Agency Service", 999900},     // $9,999
	{"instant-site", "Instant Site", 50000},  // $500
	{"enterprise", "Enterprise", 999900},     // $9,999
}

// EnsureAgencyCatalog idempotently guarantees the agency org's per-org store
// carries the agency plan listings at their catalog prices. Semantics:
//
//   - create-if-absent: a listing that already exists (e.g. an admin edited its
//     price) is NEVER overwritten; only missing plans are added.
//   - self-healing: safe to call on every boot, so the catalog is restored even
//     if the per-org store was lost (the store lives in per-org storage the
//     checkout reads).
//   - fail-open on error: the caller logs; a seed failure must not crash boot.
//     An absent catalog fails the checkout CLOSED (400), never a mispriced mint.
func EnsureAgencyCatalog(ctx context.Context) error {
	db := datastore.NewNamespaced(nscontext.WithNamespace(ctx, agencyCatalogOrg))

	// Resolve the org's default (first) store — the one the checkout prices from
	// (mirrors loadOrgCatalog in api/checkout). s is bound to the per-org
	// datastore so Put persists there.
	s := store.New(db)
	var existing []store.Store
	if _, err := s.Query().All().Limit(1).GetAll(&existing); err != nil {
		return err
	}

	if len(existing) > 0 {
		if err := s.GetById(existing[0].Id()); err != nil {
			return err
		}
	} else {
		s.Name = "Hanzo Agency"
		s.Slug = "agency"
		s.Currency = currency.USD
	}

	changed := false
	for _, p := range agencyPlans {
		if _, ok := s.Listings[p.slug]; ok {
			continue // never clobber an existing (possibly admin-edited) listing
		}
		name := p.name
		price := currency.Cents(p.price)
		available := true
		s.AddListing(p.slug, store.Listing{
			Slug:      p.slug,
			Name:      &name,
			Price:     &price,
			Available: &available,
		})
		changed = true
	}
	if !changed {
		return nil
	}
	return s.Put()
}
