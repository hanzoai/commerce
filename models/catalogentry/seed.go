package catalogentry

import (
	_ "embed"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
)

// hanzoCatalogSeed is the initial platform catalog: the ten cloud-primitive
// categories and their capabilities, aligned to the single source of truth in
// hanzo.ai lib/data/cloud-primitives.ts, enriched with per-capability
// route/docs/github and public pricing + admin-only economics. It is the
// STARTING state / offline fallback; once seeded, the CMS (admin FE editing this
// catalog) is authoritative. Re-baseline by re-aligning to cloud-primitives.ts.
//
//go:embed seed/hanzo-catalog.json
var hanzoCatalogSeed []byte

// infraTiersSeed is the initial infra-tier catalog: the cloud VM plans, GPU
// tiers, and managed-datastore tiers, categories "cloud" / "gpu" / "datastore".
// It is a VERBATIM snapshot of the pricing source of truth
// (hanzoai/pricing src/models.mjs cloudPlans + gpuTiers, and datastore.json) —
// commerce is the CMS source-of-truth these tiers are read from; the pricing
// service reads them back via GET /v1/commerce/catalog?brand=infra. The
// structured per-tier spec rides Metadata (the JSON hatch), the monthly (cloud/
// datastore) or hourly (gpu) display price rides PriceCents. Re-baseline by
// re-running the pricing→commerce generator against the pricing source.
//
//go:embed seed/infra-tiers.json
var infraTiersSeed []byte

// infraCategories is the exact set of infra-tier categories. It is BOTH the
// "infra" brand's category scope (see brandCategories in projection.go, which
// surfaces these — and only these — under GET /v1/commerce/catalog?brand=infra)
// and the seed gate. One definition, one way.
var infraCategories = []string{"cloud", "gpu", "datastore"}

// SeedRow is the @hanzo/products snapshot shape (the exact CatalogEntry
// contract). `id` == `slug`; `pricingId` may be JSON null (→ "").
type SeedRow struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Category   string     `json:"category"`
	BrandColor string     `json:"brandColor"`
	IconKey    string     `json:"iconKey"`
	Slug       string     `json:"slug"`
	Route      string     `json:"route"`
	DocsUrl    string     `json:"docsUrl"`
	ApiPath    string     `json:"apiPath"`
	ApiRoute   string     `json:"apiRoute"`
	GithubUrl  string     `json:"githubUrl"`
	External   bool       `json:"external"`
	PricingId  string     `json:"pricingId"` // null → ""
	Pricing    *Pricing   `json:"pricing"`
	Private    *Economics `json:"private"`
	Brands     []string   `json:"brands"`
	Repo       string     `json:"repo"`
	Admin      bool       `json:"admin"`
	Status     string     `json:"status"`
	Gcp        string     `json:"gcp"`
}

// HanzoSeedRows returns the parsed embedded snapshot.
func HanzoSeedRows() ([]SeedRow, error) {
	var rows []SeedRow
	if err := json.DecodeBytes(hanzoCatalogSeed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SeedIfEmpty seeds the catalog only when it is currently empty — a cheap single
// count query gates the full per-row create, so it is safe to call on every
// bootstrap. Once any entry exists (seeded or CMS-created) it never re-runs, so
// CMS state stays authoritative.
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

// Seed loads the embedded @hanzo/products snapshot into db (which MUST be
// namespaced to the catalog-owning "system" org). IDEMPOTENT + NON-DESTRUCTIVE:
// an entry whose slug already exists is left UNTOUCHED so CMS edits are never
// clobbered; only missing slugs are created. Order is the snapshot index (stable
// display order within a category). Returns the number created.
func Seed(db *datastore.Datastore) (created int, err error) {
	rows, err := HanzoSeedRows()
	if err != nil {
		return 0, err
	}

	for i, r := range rows {
		slug := r.Slug
		if slug == "" {
			slug = r.ID
		}

		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", slug).Get()
		if qerr != nil {
			return created, qerr
		}
		if ok {
			continue
		}

		e := New(db)
		e.Slug = slug
		e.Name = r.Name
		e.Category = r.Category
		e.BrandColor = r.BrandColor
		e.IconKey = r.IconKey
		e.Route = r.Route
		e.DocsUrl = r.DocsUrl
		e.ApiPath = r.ApiPath
		e.ApiRoute = r.ApiRoute
		e.GithubUrl = r.GithubUrl
		e.External = r.External
		e.PricingId = r.PricingId
		e.Pricing = r.Pricing
		e.Private = r.Private
		e.Brands = r.Brands
		e.Repo = r.Repo
		e.Admin = r.Admin
		e.Status = r.Status
		e.Gcp = r.Gcp
		e.Order = i
		e.Published = true
		if err := e.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// InfraSeedRow is one infra-tier seed record: a catalog-entry addressed by a
// globally-unique slug, carrying its display price (PriceCents) and the full
// structured spec in the Metadata JSON hatch (vcpus/memoryGB/… for cloud,
// gpu/vram/price for gpu, replicas/ramGiB/…/usage for datastore). It is a
// deliberately SMALLER shape than SeedRow — infra tiers are pricing rows, not
// console modules, so they carry no iconKey/brandColor/route/apiPath.
type InfraSeedRow struct {
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	PriceCents  currency.Cents         `json:"priceCents"`
	Currency    currency.Type          `json:"currency"`
	Order       int                    `json:"order"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// InfraSeedRows returns the parsed embedded infra-tier snapshot.
func InfraSeedRows() ([]InfraSeedRow, error) {
	var rows []InfraSeedRow
	if err := json.DecodeBytes(infraTiersSeed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SeedInfra loads the embedded infra-tier snapshot into db (which MUST be
// namespaced to the catalog-owning "system" org). IDEMPOTENT + NON-DESTRUCTIVE,
// exactly like Seed: an entry whose slug already exists is left UNTOUCHED so CMS
// edits are never clobbered; only missing slugs are created. Returns the number
// created.
func SeedInfra(db *datastore.Datastore) (created int, err error) {
	rows, err := InfraSeedRows()
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", r.Slug).Get()
		if qerr != nil {
			return created, qerr
		}
		if ok {
			continue
		}
		e := New(db)
		e.Slug = r.Slug
		e.Name = r.Name
		e.Category = r.Category
		e.Description = r.Description
		e.PriceCents = r.PriceCents
		e.Currency = r.Currency
		e.Metadata = r.Metadata
		e.Order = r.Order
		e.Published = true
		if err := e.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// SeedInfraIfEmpty seeds the infra tiers only when NONE are present yet — a
// cheap per-category count gates the full per-row create, so it is safe to call
// on every bootstrap (mirrors SeedIfEmpty). Once any infra tier exists (seeded
// or CMS-edited) it never re-runs, so CMS state — including deletions — stays
// authoritative. The gate is SEPARATE from the whole-catalog SeedIfEmpty gate,
// so the infra tiers seed independently of the 95-row hanzo product snapshot.
func SeedInfraIfEmpty(db *datastore.Datastore) (created int, err error) {
	total := 0
	for _, cat := range infraCategories {
		n, cerr := Query(db).Filter("Category=", cat).Count()
		if cerr != nil {
			return 0, cerr
		}
		total += n
	}
	if total > 0 {
		return 0, nil
	}
	return SeedInfra(db)
}
