package catalogentry

import (
	"sort"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

// canonicalCategories is the ordered "Open AI Cloud" taxonomy — the exact
// labels and order the console renders (mirrors console2 products/brand-scope
// categoryOrder). A catalog entry's Category must be one of these.
var canonicalCategories = []string{
	"AI", "Compute", "Training", "Data", "Network", "Security",
	"Observe", "Platform", "Dev", "Web3", "Apps", "Commerce", "Settings",
}

// brandCategories restricts which categories a brand's console surfaces, in
// display order. nil = all categories (hanzo). Mirrors console2 BRAND_CATEGORIES.
var brandCategories = map[string][]string{
	"hanzo": nil,
	"lux":   {"Web3", "Network", "Security", "Dev", "Settings"},
	"zoo":   {"Web3", "Network", "Security", "Dev", "Settings"},
	"pars":  {"Web3", "Network", "Security", "Dev", "Settings"},
}

// Category is one taxonomy entry in the projection.
type Category struct {
	ID    string `json:"id"`    // slugified label, e.g. "ai"
	Label string `json:"label"` // "AI"
	Order int    `json:"order"` // display rank
}

// Item is the public projection of a CatalogEntry — the exact shape the catalog
// client (@hanzo/products) consumes. Presentation keys (iconKey, brandColor)
// are strings resolved client-side; pricing is native.
type Item struct {
	ID          string         `json:"id"` // == Slug (stable path segment)
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Gcp         string         `json:"gcp,omitempty"`
	IconKey     string         `json:"iconKey"`
	BrandColor  string         `json:"brandColor,omitempty"`
	DocsUrl     string         `json:"docsUrl,omitempty"`
	ApiPath     string         `json:"apiPath,omitempty"`
	Status      string         `json:"status"`
	Repo        string         `json:"repo,omitempty"`
	Admin       bool           `json:"admin,omitempty"`
	PriceCents  currency.Cents `json:"priceCents"`
	Currency    currency.Type  `json:"currency"`
	Order       int            `json:"order"`
	ProductId   string         `json:"productId,omitempty"`
}

// Catalog is the full projection returned by GET /v1/commerce/catalog.
type Catalog struct {
	Brand      string     `json:"brand"`
	Categories []Category `json:"categories"`
	Products   []Item     `json:"products"`
}

// CategorySlug slugifies a category label deterministically (matches the
// console's categorySlug: lowercased, spaces→hyphens).
func CategorySlug(label string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(label)), " ", "-")
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

// Project reads the published catalog entries for a brand from db (which MUST be
// namespaced to the catalog-owning org) and returns the brand-scoped projection:
// the ordered taxonomy + the entries, sorted by (category order, entry order,
// name). Entries in categories the brand does not surface are dropped.
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
	if _, err := Query(db).Filter("Brand=", brand).GetAll(&entries); err != nil {
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
		cur := e.Currency
		if cur == "" {
			cur = "usd"
		}
		items = append(items, Item{
			ID:          e.Slug,
			Slug:        e.Slug,
			Name:        e.Name,
			Description: e.Description,
			Category:    e.Category,
			Gcp:         e.Gcp,
			IconKey:     e.IconKey,
			BrandColor:  e.BrandColor,
			DocsUrl:     e.DocsUrl,
			ApiPath:     e.ApiPath,
			Status:      e.Status,
			Repo:        e.Repo,
			Admin:       e.Admin,
			PriceCents:  e.PriceCents,
			Currency:    cur,
			Order:       e.Order,
			ProductId:   e.ProductId,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := catRank[items[i].Category], catRank[items[j].Category]
		if ri != rj {
			return ri < rj
		}
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		return items[i].Name < items[j].Name
	})

	return Catalog{Brand: brand, Categories: cats, Products: items}, nil
}
