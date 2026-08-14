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

// ensoModelsSeed is the initial Enso family: the five SKUs the enso service
// serves, at the retail the service BILLS. It exists because retail is the one
// number a sync may not write — UpsertModels strips Price by design — so the
// family's opening prices have to arrive as a seeded admin decision, after
// which admin.hanzo.ai owns them like any other.
//
// The numbers are transcribed from the deployed billing catalog
// (hanzoai/zen catalog-enso.yaml, mounted via universe
// charts/app/values/enso/enso.yaml), which is what production has charged since
// the 2026-07-22/23 reprice. That is the WHOLE point: the published price now
// starts life equal to the billed price instead of being typed independently.
// Pre-reprice figures (20/60, 2/6, 40/120) are stale and must not be restored.
//
// enso-vl and enso-vl-pro are seeded UNPUBLISHED. They are internal vision
// engines the family reaches through vision_fallback and are marked "NEVER
// listed SKUs" in that catalog, so they carry their rates — a price the billing
// history can reference — without appearing on the public price list.
//
//go:embed seed/enso-models.json
var ensoModelsSeed []byte

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
	Kind       string     `json:"kind"` // service (default) | client
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
		e.Role = r.Kind
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

// Correct sets every existing entry's API ADDRESS to the one the snapshot
// declares — ApiPath, its host-qualified spelling ApiRoute, and Role (kind).
// Rows the snapshot does not name are left alone, and no row is created here:
// birth is [Seed]'s job, and this is only ever a correction.
//
// # Why an address is not a merchandising choice
//
// Everything else on a row — name, price, order, status, whether it is published
// — is a decision a human makes in admin.hanzo.ai, and Seed is non-destructive
// precisely so a deploy can never overwrite one. An address is not that kind of
// value. Either the fleet answers at that path or it does not, and the fleet is
// the only thing that knows; a catalog that disagrees is not expressing a
// preference, it is wrong.
//
// It was wrong for a long time, and nothing could say so. Production first booted
// on a 95-row snapshot, the snapshot was later cut to 60, and because BOTH gates
// only ever create, the dropped rows stayed in the store forever carrying whatever
// address they were born with. Thirty-one of the 84 products a customer sees named
// a path that answers 404 — most of them literally "/v1/" + the slug, a guess
// never once checked against the router. This list was the estate's last
// hand-copied claim about its own routes: the published document is derived from
// the router and gated on prose, so a served route CANNOT be missing from it, and
// only this list could drift.
//
// So an address now travels the way the plan catalog's prices already do (see
// runPlansSeed) — as a DEPLOY, reviewed like a deploy, rather than as a script
// someone runs against production holding a SuperAdmin bearer. And cloud's
// catalog_address_test.go refuses to build a fleet whose snapshot names a path
// that fleet does not serve, so what arrives here has already been read back
// against the document it is a claim about.
func Correct(db *datastore.Datastore) (corrected int, err error) {
	rows, err := HanzoSeedRows()
	if err != nil {
		return 0, err
	}

	for _, r := range rows {
		slug := r.Slug
		if slug == "" {
			slug = r.ID
		}

		e := New(db)
		ok, qerr := e.Query().Filter("Slug=", slug).Get()
		if qerr != nil {
			return corrected, qerr
		}
		if !ok || (e.ApiPath == r.ApiPath && e.ApiRoute == r.ApiRoute && e.Role == r.Kind) {
			continue
		}

		e.ApiPath = r.ApiPath
		e.ApiRoute = r.ApiRoute
		e.Role = r.Kind
		if err := e.Update(); err != nil {
			return corrected, err
		}
		corrected++
	}
	return corrected, nil
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

// ModelSeedRow is one first-party model's opening state: the descriptor a
// family owns plus the rate vector carrying BOTH the upstream cost and the
// retail price. It is deliberately not ModelRow — a ModelRow is what a SYNCER
// publishes and may state only cost, whereas a seed states the initial admin
// decision and therefore may state price.
type ModelSeedRow struct {
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Published   bool      `json:"published"`
	Order       int       `json:"order"`
	Spec        ModelSpec `json:"spec"`
	Rates       []Rate    `json:"rates"`
}

// EnsoSeedRows returns the parsed embedded Enso family snapshot.
func EnsoSeedRows() ([]ModelSeedRow, error) {
	var rows []ModelSeedRow
	if err := json.DecodeBytes(ensoModelsSeed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SeedEnsoModels loads the embedded Enso family into db (which MUST be
// namespaced to the catalog-owning "system" org). IDEMPOTENT + NON-DESTRUCTIVE
// exactly like Seed and SeedInfra: a slug that already exists is left UNTOUCHED,
// so a price an admin has since edited is never reset to the seed. Only missing
// slugs are created. Returns the number created.
func SeedEnsoModels(db *datastore.Datastore) (created int, err error) {
	rows, err := EnsoSeedRows()
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
		e.Description = r.Description
		e.Category = CategoryForFamily(r.Spec.Family)
		e.Status = StatusEnabled
		e.Spec = specPtr(r.Spec)
		e.Rates = r.Rates
		e.Order = r.Order
		e.Published = r.Published
		if err := e.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// SeedEnsoModelsIfEmpty seeds the Enso family only when NO enso row is present
// yet, so it is safe on every bootstrap (mirrors SeedInfraIfEmpty). Gated on the
// enso category alone — per-family, so seeding Enso is independent of Zen, of
// the third-party mirror, and of the product catalog. Once any enso row exists
// it never re-runs, leaving CMS state (including deletions) authoritative.
func SeedEnsoModelsIfEmpty(db *datastore.Datastore) (created int, err error) {
	n, cerr := Query(db).Filter("Category=", CategoryEnso).Count()
	if cerr != nil {
		return 0, cerr
	}
	if n > 0 {
		return 0, nil
	}
	return SeedEnsoModels(db)
}
