package catalogentry

import (
	"cmp"
	"slices"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

// canonicalCategories is the ordered "Open AI Cloud" taxonomy — the exact ten
// categories and order the cloud-primitives single source of truth
// (hanzo.ai lib/data/cloud-primitives.ts) renders. A catalog entry's Category
// must be one of these.
var canonicalCategories = []string{
	"AI", "Compute", "Data", "Network", "Security",
	"Dev", "Platform", "Observe", "Web3", "Apps",
}

// brandCategories restricts which categories a brand's console surfaces, in
// display order. nil = all categories (hanzo). The server scopes by CATEGORY
// (matching cloud-primitives catalogForBrand), NOT by a per-entry brands list.
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
	// deliberately absent here — they ride only the super-admin ListEntries
	// surface, never this public projection.
	Pricing *Pricing `json:"pricing,omitempty"`
}

// Catalog is the full projection returned by GET /v1/commerce/catalog.
// `products` is the CatalogEntry[] @hanzo/products consumes; brand + categories
// are additive envelope fields.
type Catalog struct {
	Brand      string     `json:"brand"`
	Categories []Category `json:"categories"`
	Products   []Item     `json:"products"`
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

// Project reads the published catalog entries from db (which MUST be namespaced
// to the catalog-owning "system" org) and returns the brand-scoped projection:
// the ordered taxonomy + the entries whose CATEGORY the brand surfaces, sorted
// by (category order, entry order, name). Scoping is by category only — matching
// @hanzo/products catalogForBrand — so the same store serves every brand.
func Project(db *datastore.Datastore, brand string) (Catalog, error) {
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
		return Catalog{}, err
	}

	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		if !e.Published {
			continue
		}
		if _, ok := catRank[e.Category]; !ok {
			continue // category not surfaced by this brand
		}
		var pricingId *string
		if e.PricingId != "" {
			pid := e.PricingId
			pricingId = &pid
		}
		items = append(items, Item{
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
			// Public pricing only — Private (cost/margin) is never projected here.
			Pricing: e.Pricing,
		})
	}

	slices.SortStableFunc(items, func(a, b Item) int {
		return cmp.Or(
			cmp.Compare(catRank[a.Category], catRank[b.Category]),
			cmp.Compare(a.Order, b.Order),
			cmp.Compare(a.Name, b.Name),
		)
	})

	return Catalog{Brand: brand, Categories: cats, Products: items}, nil
}
