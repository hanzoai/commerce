// Package catalogentry is the CMS source-of-truth for a brand's OWN product
// catalog — the single list that docs.<brand>, the console product sidebar, the
// discover/overview surfaces, and pricing all derive from. commerce owns it so
// that a product's catalog identity (category, icon, docs, api path) and its
// PRICE live in one place — pricing stops being a separate service.
//
// One CatalogEntry = one product surface (e.g. "Models", "Vector", "KMS"). It
// carries CMS-editable presentation meta (iconKey/brandColor/docsUrl/apiPath)
// plus native pricing (priceCents/currency). Entries are addressed by Slug (the
// stable path segment, e.g. "models"), brand- and category-scoped, and edited
// through the standard admin CRUD (the Medusa-parity admin FE).
//
// iconKey is a STRING (e.g. "Brain") — NOT a component. The consuming client
// (@hanzo/products) maps iconKey → its @hanzogui icon component and brandColor
// token → CSS. commerce stays presentation-agnostic; it stores the keys, the
// client resolves them.
package catalogentry

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[CatalogEntry]("catalog-entry") }

// Status enumerates a product's honest enablement state (mirrors the console
// registry): a working in-console module, a live external surface, or a
// primitive that ships with no console surface yet. No fabricated states.
const (
	StatusEnabled  = "enabled"  // in-console module that works
	StatusExternal = "external" // live external brand surface
	StatusSoon     = "soon"     // primitive shipped, no console surface yet
)

// CatalogEntry is one product in the platform catalog.
type CatalogEntry struct {
	mixin.Model[CatalogEntry]

	// Slug is the stable id / base path segment, e.g. "models". Unique per
	// brand; the entry is addressed by it (GetById resolves slug).
	Slug string `json:"slug"`

	// Brand scopes the entry (hanzo/lux/zoo/pars). The catalog projection is
	// filtered by the requesting brand so each console shows only its catalog.
	Brand string `json:"brand" orm:"default:hanzo"`

	Name        string `json:"name"`                             // display label, e.g. "Models"
	Description string `json:"description" datastore:",noindex"` // one-line
	Category    string `json:"category"`                         // AI/Compute/…/Settings
	Gcp         string `json:"gcp,omitempty"`                    // GCP product it stands in for

	// Presentation meta (CMS-editable, resolved by the client).
	IconKey    string `json:"iconKey"`              // e.g. "Brain" — a @hanzogui icon name
	BrandColor string `json:"brandColor,omitempty"` // hex ("#7C3AED") or design token string
	DocsUrl    string `json:"docsUrl,omitempty"`    // canonical docs deep link
	ApiPath    string `json:"apiPath,omitempty"`    // "/v1/…" — the product's API root

	Status string `json:"status" orm:"default:enabled"` // enabled|external|soon
	Repo   string `json:"repo,omitempty"`               // source repo, e.g. "hanzoai/ai"
	Admin  bool   `json:"admin,omitempty"`              // admin-gated surface

	// Native pricing (this is why catalog=commerce: price is not a separate
	// service). PriceCents==0 means free/usage-metered (no fixed price).
	PriceCents currency.Cents `json:"priceCents"`
	Currency   currency.Type  `json:"currency" orm:"default:usd"`

	// Order is the display rank within a category (lower first).
	Order int `json:"order"`

	// Published gates an entry from the public projection without deleting it.
	Published bool `json:"published" orm:"default:true"`

	// ProductId optionally links this catalog surface to a real commerce
	// product for checkout/subscription. Empty = presentation/pricing only.
	ProductId string `json:"productId,omitempty"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (e *CatalogEntry) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(e, ps); err != nil {
		return err
	}
	if len(e.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(e.Metadata_), &e.Metadata)
	}
	return err
}

func (e *CatalogEntry) Save() ([]datastore.Property, error) {
	e.Metadata_ = string(json.EncodeBytes(&e.Metadata))
	return datastore.SaveStruct(e)
}

func New(db *datastore.Datastore) *CatalogEntry {
	e := new(CatalogEntry)
	e.Init(db)
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("catalog-entry")
}
