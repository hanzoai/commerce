// Package catalogentry is the CMS source-of-truth for the platform product
// catalog — the single list docs.<brand>, the console sidebar, and pricing all
// derive from. commerce owns the DATA (source + seed + edits); the shape is the
// @hanzo/products contract (that package owns the schema + the iconKey→component
// and brandColor→css code-maps). Pricing is native: an entry references a
// pricing plan by key (PricingId), or carries a fixed PriceCents.
//
// Conformance (GET /v1/commerce/catalog → the @hanzo/products CatalogEntry):
//   - iconKey  is a @hanzogui/lucide-icons-2 export NAME ("Brain") — never a component.
//   - brandColor is a swatch KEY ("violet") — never hex. @hanzo/products maps key→css.
//   - category is EXACTLY one of the 10 canonical categories (others are dropped by scope).
//   - route is "/<slug>", apiPath is /v1-prefixed, docsUrl is /docs/services/<slug>.
//   - pricingId is a pricing plans/<key>.json key, or null.
//   - brands is a category-derived convenience — NOT a hand-authored filter; the
//     server scopes by CATEGORY (categoriesForBrand), matching @hanzo/products
//     catalogForBrand.
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

// A catalog entry either SERVES an API or CONSUMES one. The CLI, the SDKs, the
// IDE and the console are products in their own right with no route of their
// own, so reading their health off an apiPath is a category error — it is how
// seven working products came to be advertised against paths that 404. An empty
// kind reads as KindService, because every row written before this field exists
// is API-backed.
const (
	KindService = "service" // API-backed: ApiPath is a real route
	KindClient  = "client"  // consumes the API, so carries no ApiPath
)

// KindOf reports an entry's kind, defaulting the unset field to KindService so
// a stored row and a projected one always agree on what the entry is.
func KindOf(e *CatalogEntry) string {
	if e.Role == "" {
		return KindService
	}
	return e.Role
}

// Pricing is the PUBLIC pricing block for a capability — projected to everyone.
// PublicPrice is a human display string ("From $0.10 / 1M input tokens"), or the
// literal "TODO" when a real number is not yet sourced (never a fabricated one).
// PlanTiers are subscription plan keys (api/billing/plans/<key>.json) the
// capability is included in; UsageMeter is the metering unit ("per_mtok").
type Pricing struct {
	PublicPrice string   `json:"publicPrice"`
	PlanTiers   []string `json:"planTiers,omitempty"`
	UsageMeter  string   `json:"usageMeter,omitempty"`
}

// Economics is the PRIVATE, admin-only unit economics for a capability. It is
// NEVER included in the public projection — only the super-admin ListEntries
// surface returns it. Cost is a display string ("$0.00893 / hour …") or "TODO";
// MarginPct is the gross margin percent, nil when not yet computed (so an unknown
// margin is an absent field, never a fabricated 0).
type Economics struct {
	Cost      string   `json:"cost"`
	MarginPct *float64 `json:"marginPct,omitempty"`
}

// CatalogEntry is one product in the platform catalog. Slug (== id) is the
// stable, globally-unique key the entry is addressed by.
type CatalogEntry struct {
	mixin.Model[CatalogEntry]

	Slug        string `json:"slug"`                             // stable id / path segment, e.g. "gateway"
	Name        string `json:"name"`                             // "Gateway"
	Category    string `json:"category"`                         // one of the 10 canonical categories
	Description string `json:"description" datastore:",noindex"` // one-line (additive; not in the core contract)
	Gcp         string `json:"gcp,omitempty"`                    // GCP product it stands in for

	// Presentation KEYS (CMS-editable; @hanzo/products resolves them).
	IconKey    string `json:"iconKey"`    // lucide export name, e.g. "Network"
	BrandColor string `json:"brandColor"` // swatch key, e.g. "blue"

	Route     string `json:"route"`               // marketing "/<slug>"
	DocsUrl   string `json:"docsUrl"`             // https://docs.hanzo.ai/docs/services/<slug>
	ApiPath   string `json:"apiPath"`             // /v1-prefixed path, "/v1/<slug>"
	ApiRoute  string `json:"apiRoute,omitempty"`  // host-qualified "api.hanzo.ai/v1/<slug>"
	GithubUrl string `json:"githubUrl,omitempty"` // source runtime repo URL
	External  bool   `json:"external,omitempty"`  // leaf that links out to another brand surface

	// Pricing is the public pricing block; Private is the admin-only unit
	// economics (cost + margin) — stored as noindex blobs, projected apart:
	// Pricing rides the public projection, Private only the super-admin list.
	Pricing  *Pricing   `json:"pricing,omitempty" datastore:"-"`
	Pricing_ string     `json:"-" datastore:",noindex"`
	Private  *Economics `json:"private,omitempty" datastore:"-"`
	Private_ string     `json:"-" datastore:",noindex"`

	// Rates is the entry's price VECTOR — the one way a price is expressed. A
	// model charges per input/output/cache component and per context rung; a VM
	// charges one amount per month. Both are Rates; the VM is the one-element
	// case. Each rate carries the upstream COST (synced) and the retail PRICE
	// (set in admin), so margin is derived and displayed per component.
	//
	// PriceCents/CostCents below are the LEGACY SCALAR form, still written by
	// the infra-tier seed and read by the pricing service. Every projection goes
	// through RatesOf, which synthesizes the one-element vector from them, so a
	// reader has exactly one shape regardless of which form a row was written
	// in. New writers use Rates.
	Rates  []Rate `json:"rates,omitempty" datastore:"-"`
	Rates_ string `json:"-" datastore:",noindex"`

	// Markup is the retail multiple over a synced upstream cost, as an exact
	// decimal string ("1.20"), applied to any rate that carries no explicit
	// Price. Empty ⇒ DefaultMarkup. It is PER ENTRY and editable — never a
	// global constant buried in a service's env, which is a margin nobody can
	// read.
	Markup string `json:"markup,omitempty"`

	// Spec is the model DESCRIPTOR + routing policy, present only on model rows
	// (see model.go). Stored as a noindex blob like Pricing/Private.
	Spec  *ModelSpec `json:"spec,omitempty" datastore:"-"`
	Spec_ string     `json:"-" datastore:",noindex"`

	// PricingId references a pricing plan by key (plans/<key>.json); empty ⇒
	// projected as JSON null. PriceCents is an optional native fixed-price
	// override (additive to the contract) — the PUBLIC price a customer pays.
	PricingId  string         `json:"pricingId"`
	PriceCents currency.Cents `json:"priceCents,omitempty"`
	Currency   currency.Type  `json:"currency,omitempty"`

	// CostCents is the platform's own unit cost for this product (what Hanzo
	// pays upstream) and MarginPct is the target gross margin over it. Both are
	// administrative economics — the margin surface admin.hanzo.ai edits (HIP-0106)
	// — and are projected ONLY by the owner=="admin" admin catalog, NEVER by the
	// public projection. When MarginPct is unset the admin projection derives it
	// from (PriceCents-CostCents)/PriceCents so a stored override and a computed
	// value read the same way.
	CostCents currency.Cents `json:"costCents,omitempty"`
	MarginPct float64        `json:"marginPct,omitempty"`

	// Role is service|client (KindOf defaults it). It is spelled `kind`
	// everywhere it is observable — in storage and on the wire; the Go field
	// cannot be Kind because Model[T] promotes a Kind() method that a field of
	// that name shadows, silently breaking mixin.Entity (see entity_guard.go).
	Role   string `json:"kind,omitempty"`
	Status string `json:"status" orm:"default:enabled"` // enabled|external|soon
	Repo   string `json:"repo,omitempty"`               // source repo, e.g. "hanzoai/ai"
	Admin  bool   `json:"admin,omitempty"`              // admin-gated surface

	// Brands is a category-derived convenience preserved from the seed (the
	// server filters by category, not by this list). Stored as a noindex blob.
	Brands  []string `json:"brands,omitempty" datastore:"-"`
	Brands_ string   `json:"-" datastore:",noindex"`

	Order     int  `json:"order"`                        // display rank within a category
	Published bool `json:"published" orm:"default:true"` // gates from the public projection

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
	if len(e.Brands_) > 0 {
		if err = json.DecodeBytes([]byte(e.Brands_), &e.Brands); err != nil {
			return err
		}
	}
	if len(e.Pricing_) > 0 {
		if err = json.DecodeBytes([]byte(e.Pricing_), &e.Pricing); err != nil {
			return err
		}
	}
	if len(e.Private_) > 0 {
		if err = json.DecodeBytes([]byte(e.Private_), &e.Private); err != nil {
			return err
		}
	}
	if len(e.Rates_) > 0 {
		if err = json.DecodeBytes([]byte(e.Rates_), &e.Rates); err != nil {
			return err
		}
	}
	if len(e.Spec_) > 0 {
		if err = json.DecodeBytes([]byte(e.Spec_), &e.Spec); err != nil {
			return err
		}
	}
	if len(e.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(e.Metadata_), &e.Metadata)
	}
	return err
}

func (e *CatalogEntry) Save() ([]datastore.Property, error) {
	e.Brands_ = string(json.EncodeBytes(&e.Brands))
	e.Metadata_ = string(json.EncodeBytes(&e.Metadata))
	e.Pricing_ = ""
	if e.Pricing != nil {
		e.Pricing_ = string(json.EncodeBytes(e.Pricing))
	}
	e.Private_ = ""
	if e.Private != nil {
		e.Private_ = string(json.EncodeBytes(e.Private))
	}
	e.Rates_ = ""
	if len(e.Rates) > 0 {
		e.Rates_ = string(json.EncodeBytes(&e.Rates))
	}
	e.Spec_ = ""
	if e.Spec != nil {
		e.Spec_ = string(json.EncodeBytes(e.Spec))
	}
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
