package catalogentry

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

// canonicalCategories is the ordered "Open AI Cloud" taxonomy — the exact
// labels and order @hanzo/products CATEGORY_ORDER renders. A catalog entry's
// Category must be one of these.
var canonicalCategories = []string{
	"AI", "Compute", "Data", "Network", "Security",
	"Dev", "Platform", "Observe", "Web3", "Apps",
}

// brandCategories restricts which categories a brand's console surfaces, in
// display order. nil = all categories (hanzo). Mirrors @hanzo/products
// BRAND_CATEGORIES exactly — the server scopes by CATEGORY (matching
// catalogForBrand), NOT by a per-entry brands list.
var brandCategories = map[string][]string{
	"hanzo": nil,
	"lux":   {"Web3", "Network", "Security", "Dev"},
	"zoo":   {"Web3", "Network", "Security", "Dev"},
	"pars":  {"Web3", "Network", "Security", "Dev"},
}

// Category is one taxonomy entry in the projection.
type Category struct {
	ID    string `json:"id"`    // slugified label, e.g. "ai"
	Label string `json:"label"` // "AI"
	Order int    `json:"order"` // display rank
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
	Description string         `json:"description,omitempty"`
	Gcp         string         `json:"gcp,omitempty"`
	Status      string         `json:"status,omitempty"`
	Repo        string         `json:"repo,omitempty"`
	External    bool           `json:"external,omitempty"`
	Admin       bool           `json:"admin,omitempty"`
	PriceCents  currency.Cents `json:"priceCents,omitempty"`
	Currency    currency.Type  `json:"currency,omitempty"`
	Order       int            `json:"order,omitempty"`
	ProductId   string         `json:"productId,omitempty"`

	// Pricing is the PUBLIC pricing block. Private economics (cost/margin) are
	// deliberately absent here — they ride only the owner=="admin" admin
	// projection (AdminItem), never this public projection.
	Pricing *Pricing `json:"pricing,omitempty"`
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

// AdminItem is the admin projection of a CatalogEntry: the full public Item PLUS
// the administrative economics (cost + margin) the public projection withholds.
// Only the owner=="admin" admin catalog emits it, so upstream cost and target
// margin never reach a public reader.
type AdminItem struct {
	Item
	CostCents currency.Cents `json:"costCents"`
	MarginPct float64        `json:"marginPct"`
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
		out = append(out, Category{ID: CategorySlug(label), Label: label, Order: i})
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
		Status:      e.Status,
		Repo:        e.Repo,
		External:    e.External,
		Admin:       e.Admin,
		PriceCents:  e.PriceCents,
		Currency:    e.Currency,
		Order:       e.Order,
		ProductId:   e.ProductId,
		// Public pricing block only — Private (cost/margin) is never projected here.
		Pricing: e.Pricing,
	}
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
			Item:      item(e),
			CostCents: e.CostCents,
			MarginPct: pct,
		})
	}
	return AdminCatalog{Brand: brand, Categories: cats, Products: items}, nil
}
