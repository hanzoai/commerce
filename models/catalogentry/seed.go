package catalogentry

import (
	_ "embed"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
)

// hanzoCatalogSeed is the initial Hanzo platform catalog, extracted from the
// console product registry (console2 src/lib/products/registry.tsx). It is the
// STARTING state — the CMS (admin FE editing this catalog) is authoritative
// thereafter. Regenerate only to re-baseline from the console registry.
//
//go:embed seed/hanzo-catalog.json
var hanzoCatalogSeed []byte

// SeedRow is the on-disk seed shape (a superset of the projection Item — it also
// carries brand/published/currency, which the projection derives or filters).
type SeedRow struct {
	Slug        string         `json:"slug"`
	Brand       string         `json:"brand"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Gcp         string         `json:"gcp"`
	IconKey     string         `json:"iconKey"`
	BrandColor  string         `json:"brandColor"`
	DocsUrl     string         `json:"docsUrl"`
	ApiPath     string         `json:"apiPath"`
	Status      string         `json:"status"`
	Repo        string         `json:"repo"`
	Admin       bool           `json:"admin"`
	PriceCents  currency.Cents `json:"priceCents"`
	Currency    currency.Type  `json:"currency"`
	Order       int            `json:"order"`
	Published   bool           `json:"published"`
}

// HanzoSeedRows returns the parsed embedded Hanzo catalog seed.
func HanzoSeedRows() ([]SeedRow, error) {
	var rows []SeedRow
	if err := json.DecodeBytes(hanzoCatalogSeed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SeedIfEmpty seeds the catalog only when it is currently empty — a cheap
// single count query gates the full per-row upsert, so it is safe to call on
// every bootstrap without paying the seed cost once populated. Once any entry
// exists (seeded or CMS-created), it never re-runs, so CMS state is authoritative.
func SeedIfEmpty(db *datastore.Datastore) (created int, err error) {
	n, err := Query(db).Count()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, nil
	}
	return Seed(db)
}

// Seed upserts the embedded Hanzo catalog into db (which MUST be namespaced to
// the catalog-owning org). It is IDEMPOTENT and NON-DESTRUCTIVE: an entry that
// already exists (matched by slug+brand) is left UNTOUCHED so CMS edits are
// never clobbered by a re-seed; only missing entries are created. Returns the
// number of entries created.
func Seed(db *datastore.Datastore) (created int, err error) {
	rows, err := HanzoSeedRows()
	if err != nil {
		return 0, err
	}

	for _, r := range rows {
		// Skip if an entry with this slug+brand already exists — never overwrite
		// CMS edits.
		existing := New(db)
		ok, qerr := existing.Query().
			Filter("Slug=", r.Slug).
			Filter("Brand=", r.Brand).
			Get()
		if qerr != nil {
			return created, qerr
		}
		if ok {
			continue
		}

		e := New(db)
		e.Slug = r.Slug
		e.Brand = r.Brand
		e.Name = r.Name
		e.Description = r.Description
		e.Category = r.Category
		e.Gcp = r.Gcp
		e.IconKey = r.IconKey
		e.BrandColor = r.BrandColor
		e.DocsUrl = r.DocsUrl
		e.ApiPath = r.ApiPath
		e.Status = r.Status
		e.Repo = r.Repo
		e.Admin = r.Admin
		e.PriceCents = r.PriceCents
		e.Currency = r.Currency
		e.Order = r.Order
		e.Published = r.Published
		if err := e.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
