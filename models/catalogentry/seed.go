package catalogentry

import (
	_ "embed"
	"slices"

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

// ensoModelsSeed is the Enso family: the six SKUs the enso service serves, at
// the price the model API bills. Each paid SKU sells at ONE fixed price
// (hanzoai/ai controllers/rates.go ensoPrices, owner 2026-10-15): enso-flash
// 0.33 / 1.65, enso-pro 4.40 / 22.00, enso-ultra 5.50 / 27.50 a million tokens,
// with cache reads and writes in the same multiple. enso is the router over the
// three (hanzoai/ai v1.833.349), billed at the tier that answers and listed at a
// request's that states no depth, enso-flash's. Retail
// is the one number a sync may not write — UpsertModels strips Price by design —
// so the prices arrive here, and [Reprice] reads them back over the store on
// every boot. It carries no cost: cost comes only from a sync.
//
// enso-vl and enso-vl-pro are seeded UNPUBLISHED. They are internal vision
// engines the family reaches through its vision fallback, never listed SKUs, so
// they carry their rates — a price the billing history can reference — without
// appearing on the public price list.
//
//go:embed seed/enso-models.json
var ensoModelsSeed []byte

// modelsSeed is every other model the model API charges for that no family
// prices: decisions (Kai, Jev, Zen answering one), speech, image and video
// generation, embeddings. These rates were tables in hanzoai/ai's code; they are
// rows here, at the prices production billed, and an admin owns each price from
// the row's birth (no reprice). A model none of the catalog's rows names is not
// sold. Seeded unpublished except Kai: they price calls, not the public list.
//
//go:embed seed/hanzo-models.json
var modelsSeed []byte

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
		// Kind, not the raw field: a stored "service" and a snapshot that says
		// nothing are the SAME kind, and comparing the strings would rewrite the
		// row on every boot forever. Twice is once — correct_test.go pins it.
		if !ok || (e.ApiPath == r.ApiPath && e.ApiRoute == r.ApiRoute && KindOf(e) == Kind(r.Kind)) {
			continue
		}

		e.ApiPath = r.ApiPath
		e.ApiRoute = r.ApiRoute
		e.Role = Kind(r.Kind)
		if err := e.Update(); err != nil {
			return corrected, err
		}
		corrected++
	}
	return corrected, nil
}

// Rename moves every entry sitting in a RETIRED category to the one that
// replaced it, per renamedCategories (projection.go). It returns the number
// moved and is idempotent: a second run finds nothing, because the first left
// no row carrying a retired label.
//
// A category rename is not the same act as an address correction, and this is
// not [Correct] with a wider reach. Correct rewrites a field on rows that are
// working fine; this one only ever touches rows that are ALREADY broken. The
// canonical list is what scoped() filters on, so the instant a label leaves it
// every row still naming it stops being projected — the products do not move,
// they disappear. Reading the old label is therefore never a preference to
// respect; it is a row that has been orphaned by a taxonomy change and has
// exactly one right home.
//
// That narrowness is the safety property. Which of the ten categories a product
// belongs to IS a merchandising decision, made in admin.hanzo.ai, and this
// cannot overwrite one: a row in a canonical category never matches the map.
//
// Like every other catalog reconciliation here, it travels as a DEPLOY rather
// than as a script someone runs against production holding a SuperAdmin bearer.
func Rename(db *datastore.Datastore) (moved int, err error) {
	if len(renamedCategories) == 0 {
		return 0, nil
	}

	// Two steps, and the split is not stylistic: GetAll fills plain structs that
	// are not bound to the store, so writing one back panics. A row is READ in
	// bulk to find out which slugs are stranded, then re-fetched through New(db)
	// to be written — the same shape [Correct] uses, for the same reason.
	entries := make([]*CatalogEntry, 0, 128)
	if _, err := Query(db).GetAll(&entries); err != nil {
		return 0, err
	}
	stranded := make(map[string]string, len(renamedCategories))
	for _, e := range entries {
		if to, retired := renamedCategories[e.Category]; retired {
			stranded[e.Slug] = to
		}
	}

	for slug, to := range stranded {
		e := New(db)
		ok, qerr := e.Query().Filter("Slug=", slug).Get()
		if qerr != nil {
			return moved, qerr
		}
		if !ok || renamedCategories[e.Category] == "" {
			continue
		}
		e.Category = to
		if err := e.Update(); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
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

// SeedEnsoModels creates each Enso row the embedded family names that db (which
// MUST be namespaced to the catalog-owning "system" org) lacks, and returns how
// many. A row that exists is left to Reprice, which owns its price.
//
// It runs on EVERY boot, because the pricing catalog is what Enso is charged at:
// the model API sells an Enso SKU only while its row is listed here, so a SKU the
// seed names and the store lacks is a SKU nobody can buy — and a SKU added to the
// seed after a store was first seeded must still reach it. To retire a SKU, take it
// out of the seed.
func SeedEnsoModels(db *datastore.Datastore) (created int, err error) {
	rows, err := EnsoSeedRows()
	if err != nil {
		return 0, err
	}
	return seedModels(db, rows)
}

// ModelSeedRows returns the parsed embedded rows of the models no family prices.
func ModelSeedRows() ([]ModelSeedRow, error) {
	var rows []ModelSeedRow
	if err := json.DecodeBytes(modelsSeed, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SeedModels creates each row of the models no family prices that db lacks, on
// every boot, and returns how many. A row that exists keeps the price an admin set.
func SeedModels(db *datastore.Datastore) (created int, err error) {
	rows, err := ModelSeedRows()
	if err != nil {
		return 0, err
	}
	return seedModels(db, rows)
}

// seedModels creates each of rows that db lacks, whole: at birth there is no human
// decision to preserve.
func seedModels(db *datastore.Datastore, rows []ModelSeedRow) (created int, err error) {
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

// Reprice sets each Enso row the seed names to the seed's price, rate by rate,
// and returns how many rows it moved. A rate the row lacks is added; a cost (a
// sync's), a name, an order, whether it is published, a row the seed does not
// name and a row an admin deleted are left alone. Prices compare as decimals, so
// "4.4" where the seed says "4.40" moves nothing, and twice is once.
//
// # Why an Enso price is no longer an admin's
//
// Everywhere else on the catalog a price is a decision a human makes in
// admin.hanzo.ai, and every seed is non-destructive so a deploy cannot undo one.
// Enso's is not that kind of value. Its price is the owner's (2026-10-15), set
// in the seed and carried by a deploy, and the model API bills each Enso SKU at
// this row — so a row that drifted from the seed is a price nobody decided. Like
// an address ([Correct]), it is read back from the place it is declared, on every
// boot, and a change to it is a deploy.
func Reprice(db *datastore.Datastore) (repriced int, err error) {
	rows, err := EnsoSeedRows()
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		e := New(db)
		ok, qerr := e.Query().Filter("Slug=", r.Slug).Get()
		if qerr != nil {
			return repriced, qerr
		}
		if !ok {
			continue
		}
		rates, moved := priced(e.Rates, r.Rates)
		if !moved {
			continue
		}
		e.Rates = rates
		if err := e.Update(); err != nil {
			return repriced, err
		}
		repriced++
	}
	return repriced, nil
}

// priced is have with each of want's prices laid over the rate of the same
// identity (key, unit, context rung) and each rate of want's that have lacks
// appended, and whether anything moved.
func priced(have, want []Rate) ([]Rate, bool) {
	out := slices.Clone(have)
	moved := false
	for _, w := range want {
		i := slices.IndexFunc(out, w.same)
		if i < 0 {
			out = append(out, w)
			moved = true
			continue
		}
		a, okA := dec(out[i].Price)
		b, okB := dec(w.Price)
		if okA && okB && a.Equal(b) {
			continue
		}
		out[i].Price = w.Price
		moved = true
	}
	return out, moved
}
