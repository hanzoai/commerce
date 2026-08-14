package catalogentry

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

// canonicalCategories is the ordered "Open AI Cloud" taxonomy, and it is the
// SOURCE of it. A catalog entry's Category must be one of these.
//
// This used to say it was "the exact labels and order @hanzo/products
// CATEGORY_ORDER renders", which pointed the arrow the wrong way and was false
// while it said so: that package rendered a `Commerce` category no row here has
// ever carried, and omitted `Dev`, which eight carry. Four hand-written copies
// of this list existed across the estate and every one of them was somebody's
// mirror of somebody else.
//
// The direction is now stated once and runs one way. This list is edited here,
// served at GET /v1/commerce/catalog, and every consumer generates its copy from
// that response — @hanzo/products by `pnpm sync` (which fails a release when its
// copy disagrees), the marketing site by scripts/sync-catalog.mjs at prebuild,
// and the console by importing the package. Nothing downstream retypes it.
var canonicalCategories = []string{
	"AI", "Compute", "Data", "Network", "Security",
	"Dev", "Infrastructure", "Observe", "Web3", "Apps",
}

// renamedCategories maps a category label the taxonomy has RETIRED to the one
// that replaced it. It exists because a label is not just a display string:
// CategorySlug derives the public URL from it, and scoped() drops any entry
// whose Category is absent from canonicalCategories. So renaming a category in
// that list, alone, would make every row still carrying the old label vanish
// from the catalog — six products in the case of Platform → Infrastructure.
//
// [Rename] applies this map to the store on boot, and it can only ever touch
// rows whose category is ALREADY invisible. A row sitting in a canonical
// category is left alone, which is what keeps this from reverting the one
// genuine merchandising decision an admin makes here: which of the ten a
// product belongs to.
//
// Infrastructure was Platform until cloud.hanzo.ai titled its products section
// "Platform"; a category of the same name nested inside a section of that name
// reads as a mistake, so the inner one took the name that describes it.
var renamedCategories = map[string]string{
	"Platform": "Infrastructure",
}

// brandCategories restricts which categories a brand's console surfaces, in
// display order. nil = all categories (hanzo). The server scopes by CATEGORY
// (matching catalogForBrand), NOT by a per-entry brands list.
//
// This is the source of the per-brand scope too, on the same footing as the
// taxonomy above: a Lux console asks GET /v1/commerce/catalog?brand=lux, so the
// answer to "which categories does lux show" is whatever this returns.
// @hanzo/products reads each brand's answer instead of keeping the second copy
// it used to keep, which had drifted to scoping the chain brands to
// Infrastructure while this scoped them to Dev.
var brandCategories = map[string][]string{
	"hanzo": nil,
	"lux":   {"Web3", "Network", "Security", "Dev"},
	"zoo":   {"Web3", "Network", "Security", "Dev"},
	"pars":  {"Web3", "Network", "Security", "Dev"},

	// "infra" is a brand-neutral SCOPE, not a customer brand: it surfaces the
	// cloud/gpu/datastore pricing tiers (categories in infraCategories) and
	// NOTHING else. These are platform infrastructure the pricing service reads
	// via GET /v1/commerce/catalog?brand=infra; they are deliberately kept OUT
	// of every per-brand console/docs catalog (hanzo/lux/zoo/pars) — their
	// category sets exclude cloud/gpu/datastore — so a pricing tier never leaks
	// into a product sidebar. infraCategories is defined once (seed.go).
	"infra": infraCategories,

	// "models" is the other brand-neutral SCOPE, on the same precedent: the
	// model catalog (our Enso and Zen families, plus everything we resell)
	// surfaced at GET /v1/commerce/catalog?brand=models and NOWHERE else, so a
	// model row never lands in a product sidebar. This is the ONE catalog every
	// surface reads. modelCategories is defined once (model.go).
	"models": modelCategories,
}

// Category is one taxonomy entry in the projection.
type Category struct {
	ID    string `json:"id"`    // slugified label, e.g. "ai"
	Label string `json:"label"` // "AI"
	Order int    `json:"order"` // display rank
	Color string `json:"color"` // accent swatch key, e.g. "violet"
}

// categoryColors is the accent each category reads as — the swatch key a surface
// resolves to css, exactly like a product's BrandColor.
//
// It is served rather than looked up per surface because a category's colour is
// part of what the category IS, not decoration each reader picks: it is the only
// cue that distinguishes ten groups at a glance in a sidebar, and a group that is
// teal in the console and indigo on the site is two different groups as far as
// anyone looking at both can tell. That had happened — console.hanzo.ai drew this
// category teal while @hanzo/products carried indigo for it — because the mapping
// existed twice and neither copy was anyone's answer.
//
// The values are the ones the console has been shipping, since that is the
// surface where a category accent is actually rendered at size (sidebar icons and
// the category tiles); nothing re-themes as a result of moving them here.
//
// Keys are the canonical labels; TestCategoryColors_CoverTheTaxonomy pins that
// this map and canonicalCategories name exactly the same set, so a category can
// neither be added without an accent nor keep one after it is retired.
var categoryColors = map[string]string{
	"AI":             "violet",
	"Compute":        "blue",
	"Data":           "cyan",
	"Network":        "sky",
	"Security":       "red",
	"Dev":            "indigo",
	"Infrastructure": "teal",
	"Observe":        "green",
	"Web3":           "amber",
	"Apps":           "pink",
}

// Item is the public projection of a CatalogEntry — the exact @hanzo/products
// CatalogEntry shape. Presentation keys (iconKey, brandColor) are strings
// resolved client-side; pricingId is null when unset. Fields beyond the core
// contract (gcp, repo, admin, status, description, priceCents, order, productId)
// are additive — the client ignores unknowns.
type Item struct {
	ID         string   `json:"id"` // == Slug (stable, unique)
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	BrandColor string   `json:"brandColor"`
	IconKey    string   `json:"iconKey"`
	Slug       string   `json:"slug"`
	Route      string   `json:"route"`
	DocsUrl    string   `json:"docsUrl"`
	ApiPath    string   `json:"apiPath"`
	ApiRoute   string   `json:"apiRoute,omitempty"`  // host-qualified api.hanzo.ai/v1/<slug>
	GithubUrl  string   `json:"githubUrl,omitempty"` // source runtime repo URL
	PricingId  *string  `json:"pricingId"`           // string OR null
	Brands     []string `json:"brands,omitempty"`    // category-derived convenience

	// Additive (client ignores unknowns).
	Description string `json:"description,omitempty"`
	Gcp         string `json:"gcp,omitempty"`

	// Kind is service|client. It is always stated, never omitted, so a reader
	// never has to infer from an empty ApiPath why a product has no route: a
	// client CONSUMES the API and must not be judged by apiPath reachability.
	Kind       string         `json:"kind"`
	Status     string         `json:"status,omitempty"`
	Repo       string         `json:"repo,omitempty"`
	External   bool           `json:"external,omitempty"`
	Admin      bool           `json:"admin,omitempty"`
	PriceCents currency.Cents `json:"priceCents,omitempty"`
	Currency   currency.Type  `json:"currency,omitempty"`
	Order      int            `json:"order,omitempty"`
	ProductId  string         `json:"productId,omitempty"`

	// Pricing is the PUBLIC pricing block. Private economics (cost/margin) are
	// deliberately absent here — they ride only the owner=="admin" admin
	// projection (AdminItem), never this public projection.
	Pricing *Pricing `json:"pricing,omitempty"`

	// Rates is the public price VECTOR — retail only, one element per metered
	// component and context rung. Every entry projects it, synthesized from the
	// legacy scalar when a row predates Rates, so a reader has one shape.
	Rates []RateView `json:"rates,omitempty"`

	// Spec is the model descriptor + routing policy, present on model rows. It
	// is projected to EVERY caller: minTier and enabled decide what a caller may
	// USE, never what they may SEE.
	Spec *ModelSpec `json:"spec,omitempty"`

	// Metadata is the structured-spec JSON hatch (the entry's Metadata map),
	// projected verbatim. Empty for console products (omitted); for the infra
	// tiers it carries the machine-readable spec (vcpus/memoryGB/… for cloud,
	// gpu/vram/price for gpu, replicas/ramGiB/…/usage for datastore) that the
	// pricing service maps back into its cloudPlans/gpuTiers/datastore shapes.
	// It is public presentation/pricing data only — never cost or margin.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Catalog is the full PUBLIC projection returned by GET /v1/commerce/catalog.
// `products` is the CatalogEntry[] @hanzo/products consumes; brand + categories
// are additive envelope fields. It carries metadata + the public price
// (Item.PriceCents) + the plan reference (Item.PricingId) + the public pricing
// block (Item.Pricing) — NEVER cost or margin.
type Catalog struct {
	Brand      string     `json:"brand"`
	Categories []Category `json:"categories"`
	Products   []Item     `json:"products"`
}

// RateView is the PUBLIC projection of one Rate: what the customer pays, per
// unit and per context rung. Cost and margin are deliberately absent — they
// ride only AdminRateView.
type RateView struct {
	Key        string `json:"key,omitempty"`
	Unit       string `json:"unit"`
	MaxContext int    `json:"maxContext,omitempty"`
	Price      string `json:"price,omitempty"`
}

// AdminRateView is the admin projection of one Rate: the retail price PLUS the
// upstream cost we pay and the resulting margin. MarginPct is a pointer so an
// uncomputable margin is an absent field, never a fabricated 0 — and a NEGATIVE
// margin is shown plainly rather than suppressed, because selling under cost is
// a decision that must be visible.
type AdminRateView struct {
	RateView
	Cost      string   `json:"cost,omitempty"`
	MarginPct *float64 `json:"marginPct,omitempty"`
}

// AdminItem is the admin projection of a CatalogEntry: the full public Item PLUS
// the administrative economics (cost + margin) the public projection withholds.
// Only the owner=="admin" admin catalog emits it, so upstream cost and target
// margin never reach a public reader.
type AdminItem struct {
	Item
	CostCents currency.Cents `json:"costCents"`
	MarginPct float64        `json:"marginPct"`

	// Markup is the entry's retail multiple over cost, and AdminRates is the
	// per-component cost/price/margin the CTO reads "on each / all".
	Markup     string          `json:"markup,omitempty"`
	AdminRates []AdminRateView `json:"adminRates,omitempty"`
}

// AdminCatalog is the projection returned by GET /v1/commerce/admin/catalog. It
// mirrors Catalog but its products carry cost + marginPct so admin.hanzo.ai can
// administrate margin.
type AdminCatalog struct {
	Brand      string      `json:"brand"`
	Categories []Category  `json:"categories"`
	Products   []AdminItem `json:"products"`
}

// marginPct returns the gross margin of a public price over a unit cost, as a
// percentage rounded to two decimals: (price-cost)/price*100. A zero price
// yields 0 (no basis to take a margin over).
func marginPct(price, cost currency.Cents) float64 {
	if price <= 0 {
		return 0
	}
	pct := float64(price-cost) / float64(price) * 100
	return math.Round(pct*100) / 100
}

// CategorySlug slugifies a category label the way @hanzo/products categorySlug
// does: simple lowercase ("AI"→"ai", "Web3"→"web3").
func CategorySlug(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// categoriesForBrand returns the ordered, brand-scoped taxonomy.
func categoriesForBrand(brand string) []Category {
	allowed, known := brandCategories[brand]
	labels := canonicalCategories
	if known && allowed != nil {
		labels = allowed
	}
	out := make([]Category, 0, len(labels))
	for i, label := range labels {
		out = append(out, Category{
			ID:    CategorySlug(label),
			Label: label,
			Order: i,
			Color: categoryColors[label],
		})
	}
	return out
}

// item is the public projection of a single entry — the one place the CatalogEntry
// → Item field map lives, shared by the public and admin projections. It carries
// only public fields (never CostCents/MarginPct).
func item(e *CatalogEntry) Item {
	var pricingId *string
	if e.PricingId != "" {
		pid := e.PricingId
		pricingId = &pid
	}
	return Item{
		ID:          e.Slug,
		Name:        e.Name,
		Category:    e.Category,
		BrandColor:  e.BrandColor,
		IconKey:     e.IconKey,
		Slug:        e.Slug,
		Route:       e.Route,
		DocsUrl:     e.DocsUrl,
		ApiPath:     e.ApiPath,
		ApiRoute:    e.ApiRoute,
		GithubUrl:   e.GithubUrl,
		PricingId:   pricingId,
		Brands:      e.Brands,
		Description: e.Description,
		Gcp:         e.Gcp,
		Kind:        KindOf(e),
		Status:      e.Status,
		Repo:        e.Repo,
		External:    e.External,
		Admin:       e.Admin,
		PriceCents:  e.PriceCents,
		Currency:    e.Currency,
		Order:       e.Order,
		ProductId:   e.ProductId,
		// Public pricing block only — Private (cost/margin) is never projected here.
		Pricing:  e.Pricing,
		Rates:    rateViews(e),
		Spec:     e.Spec,
		Metadata: e.Metadata,
	}
}

// rateViews projects an entry's price vector for a public reader: retail only.
func rateViews(e *CatalogEntry) []RateView {
	rates := RatesOf(e)
	if len(rates) == 0 {
		return nil
	}
	out := make([]RateView, 0, len(rates))
	for _, r := range rates {
		out = append(out, RateView{
			Key:        r.Key,
			Unit:       r.Unit,
			MaxContext: r.MaxContext,
			Price:      r.RetailPrice(e.Markup),
		})
	}
	return out
}

// adminRateViews projects the same vector for an admin: retail, upstream cost
// and the derived margin, per component.
func adminRateViews(e *CatalogEntry) []AdminRateView {
	rates := RatesOf(e)
	if len(rates) == 0 {
		return nil
	}
	out := make([]AdminRateView, 0, len(rates))
	for _, r := range rates {
		out = append(out, AdminRateView{
			RateView: RateView{
				Key:        r.Key,
				Unit:       r.Unit,
				MaxContext: r.MaxContext,
				Price:      r.RetailPrice(e.Markup),
			},
			Cost:      r.Cost,
			MarginPct: RateMarginPct(r, e.Markup),
		})
	}
	return out
}

// scoped reads the published catalog entries from db (which MUST be namespaced to
// the catalog-owning "system" org) and returns the brand-scoped taxonomy plus the
// entries whose CATEGORY the brand surfaces, sorted by (category order, entry
// order, name). Scoping is by category only — matching @hanzo/products
// catalogForBrand — so the same store serves every brand. Shared by Project (public)
// and ProjectAdmin (owner=="admin"): the ONE query + filter + sort, projected into
// two item shapes.
func scoped(db *datastore.Datastore, brand string) (string, []Category, map[string]int, []*CatalogEntry, error) {
	if brand == "" {
		brand = "hanzo"
	}
	cats := categoriesForBrand(brand)
	catRank := make(map[string]int, len(cats))
	for _, c := range cats {
		catRank[c.Label] = c.Order
	}
	entries := make([]*CatalogEntry, 0, 128)
	if _, err := Query(db).GetAll(&entries); err != nil {
		return brand, cats, catRank, nil, err
	}
	kept := make([]*CatalogEntry, 0, len(entries))
	for _, e := range entries {
		if !e.Published {
			continue
		}
		if _, ok := catRank[e.Category]; !ok {
			continue // category not surfaced by this brand
		}
		kept = append(kept, e)
	}
	slices.SortStableFunc(kept, func(a, b *CatalogEntry) int {
		return cmp.Or(
			cmp.Compare(catRank[a.Category], catRank[b.Category]),
			cmp.Compare(a.Order, b.Order),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return brand, cats, catRank, kept, nil
}

// Project returns the PUBLIC brand-scoped catalog: metadata + public price +
// plan reference + public pricing block, NEVER cost or margin.
func Project(db *datastore.Datastore, brand string) (Catalog, error) {
	brand, cats, _, entries, err := scoped(db, brand)
	if err != nil {
		return Catalog{}, err
	}
	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, item(e))
	}
	return Catalog{Brand: brand, Categories: cats, Products: items}, nil
}

// ProjectAdmin returns the admin brand-scoped catalog: the public projection PLUS
// cost + marginPct on every entry. Callers MUST gate this on owner=="admin"; the
// projection itself only adds the administrative economics. MarginPct falls back to
// the derived (price-cost)/price margin when the entry stores no explicit override.
func ProjectAdmin(db *datastore.Datastore, brand string) (AdminCatalog, error) {
	brand, cats, _, entries, err := scoped(db, brand)
	if err != nil {
		return AdminCatalog{}, err
	}
	items := make([]AdminItem, 0, len(entries))
	for _, e := range entries {
		pct := e.MarginPct
		if pct == 0 {
			pct = marginPct(e.PriceCents, e.CostCents)
		}
		items = append(items, AdminItem{
			Item:       item(e),
			CostCents:  e.CostCents,
			MarginPct:  pct,
			Markup:     e.Markup,
			AdminRates: adminRateViews(e),
		})
	}
	return AdminCatalog{Brand: brand, Categories: cats, Products: items}, nil
}
