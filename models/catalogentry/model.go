package catalogentry

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

// A MODEL IS A CATALOG ENTRY.
//
// It is the same kind, the same store, the same super-admin CRUD and the same
// margin projection as every other thing we sell. A second catalog concept —
// a models table, a models endpoint, a models admin page — is the exact defect
// this converges: two lists always drift, and a drifted price list is a money
// bug. What a model adds is a DESCRIPTOR (what it is and how it may be routed),
// carried in ModelSpec; its economics are the generic Rate vector every entry
// already has.

// Model families. Enso and Zen are OURS and are presented as ours — never
// described in terms of an upstream model. Everything we resell is third-party,
// named plainly by its VENDOR ("Meta", never "Meta Llama").
const (
	FamilyEnso       = "enso"
	FamilyZen        = "zen"
	FamilyThirdParty = "third-party"
)

// Model categories. Following the infra-tier precedent exactly: model rows get
// their own categories, surfaced under the brand-neutral "models" SCOPE, so
// they never pollute the ten canonical cloud-primitive product categories.
// Our two families are their own categories — that IS the split between ours
// and resold, expressed as data rather than as a naming convention.
const (
	CategoryEnso  = "enso"
	CategoryZen   = "zen"
	CategoryModel = "model" // third-party, grouped for display by ModelSpec.Vendor
)

// modelCategories is the exact set of model categories. Like infraCategories it
// is BOTH the "models" brand scope (see brandCategories) and the kind test.
// One definition, one way.
var modelCategories = []string{CategoryEnso, CategoryZen, CategoryModel}

// CategoryForFamily maps a family to the category its entries live in, so a
// syncer never has to know the taxonomy.
func CategoryForFamily(family string) string {
	switch family {
	case FamilyEnso:
		return CategoryEnso
	case FamilyZen:
		return CategoryZen
	default:
		return CategoryModel
	}
}

// IsModel reports whether an entry is a model row.
func IsModel(e *CatalogEntry) bool {
	for _, c := range modelCategories {
		if e.Category == c {
			return true
		}
	}
	return false
}

// ModelSpec is the model DESCRIPTOR: what the model is, and the routing policy
// that says exactly how it may be used. It rides one entry as a noindex blob,
// alongside the generic price vector.
//
// VISIBILITY AND ENTITLEMENT ARE DIFFERENT THINGS and this type keeps them
// apart. CatalogEntry.Published is the ONLY hide switch and exists for a
// genuinely private entry (a BYO-key model scoped to one org). Everything else
// — MinTier, Enabled, Unavailable — decides what a caller may USE and what they
// are CHARGED, never what they may SEE. A model a caller cannot yet afford is
// an upsell: it appears with its price and an honest reason, never omitted. A
// customer seeing fewer models than an anonymous visitor is the bug, not the
// feature.
type ModelSpec struct {
	Vendor string `json:"vendor,omitempty"` // plain vendor name: "Meta", never "Meta Llama"
	Family string `json:"family"`           // enso | zen | third-party

	// Routing: which family service answers this slug, and under what id
	// upstream. Routing is DATA on the one catalog — never a hardcoded pin in
	// a router, which is how a model gets listed that nothing can serve.
	Serves   string `json:"serves,omitempty"`   // enso | zen | openrouter | …
	Upstream string `json:"upstream,omitempty"` // model id at Serves, when it differs from Slug

	Modality      string   `json:"modality,omitempty"` // chat | embedding | image | audio | video | rerank
	ContextWindow int      `json:"contextWindow,omitempty"`
	MaxOutput     int      `json:"maxOutput,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"` // vision, tools, …

	// MinTier is the entitlement floor ("" | trial | paid). It gates USE, and
	// is projected to every caller so the reason is honest and legible.
	MinTier string `json:"minTier,omitempty"`

	// Enabled says the model is routable right now. A listed-but-disabled model
	// (upstream withdrawn, capacity pulled) stays VISIBLE with its price and
	// Unavailable set — deleting the row would break the billing history that
	// references it.
	Enabled     bool   `json:"enabled"`
	Unavailable string `json:"unavailable,omitempty"` // honest reason when !Enabled
}

// ModelRow is what a SYNCER publishes for one model — the seam between the
// families and commerce. The family repos (enso, zen) and the upstream
// (OpenRouter) own the STRUCTURE: which SKUs exist, what each means, and the
// per-rung shape of their cost. Commerce holds the NUMBERS and admin edits
// them.
//
// Costs is a cost-only rate vector: a syncer states what we PAY, per component
// and per context rung, and nothing else. UpsertModels enforces that.
type ModelRow struct {
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Spec        ModelSpec `json:"spec"`
	Costs       []Rate    `json:"costs,omitempty"`
	Order       int       `json:"order,omitempty"`
	Metadata    Map       `json:"metadata,omitempty"`
}

// SyncStats reports what a sync did, so a run is legible without diffing rows.
type SyncStats struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

// UpsertModels lands a syncer's view of the model catalog.
//
// THE ONE RULE, and the reason this function exists rather than a generic
// upsert: A SYNC OWNS COST, ADMIN OWNS PRICE. On an existing entry it refreshes
// the upstream cost and the machine-observable facts (what the model is, where
// it routes) and touches NOTHING else — not Price, not Markup, not MinTier, not
// Published, not Name. An upstream price move therefore surfaces as a margin
// moving in admin.hanzo.ai, and can never silently change what a customer pays.
//
// The rate SET follows upstream (a withdrawn context rung disappears, a new one
// appears) but each surviving rate keeps the Price a human put on it, matched
// by rate identity (key, unit, rung).
//
// A row for a slug that does not exist yet is CREATED whole — including its
// policy defaults — because at birth there is no human decision to preserve.
func UpsertModels(db *datastore.Datastore, rows []ModelRow) (SyncStats, error) {
	var stats SyncStats
	for _, r := range rows {
		if r.Slug == "" {
			continue
		}
		e := New(db)
		ok, err := e.Query().Filter("Slug=", r.Slug).Get()
		if err != nil {
			return stats, err
		}
		if !ok {
			e = New(db)
			e.Slug = r.Slug
			e.Name = r.Name
			e.Description = r.Description
			e.Category = CategoryForFamily(r.Spec.Family)
			e.Status = StatusEnabled
			e.Spec = specPtr(r.Spec)
			e.Rates = costOnly(r.Costs)
			e.Order = r.Order
			e.Metadata = r.Metadata
			e.Published = true
			if err := e.Create(); err != nil {
				return stats, err
			}
			stats.Created++
			continue
		}

		e.Spec = mergeSpec(e.Spec, r.Spec)
		e.Category = CategoryForFamily(e.Spec.Family)
		e.Rates = mergeRates(RatesOf(e), r.Costs)
		if len(r.Metadata) > 0 {
			e.Metadata = r.Metadata
		}
		if err := e.Update(); err != nil {
			return stats, err
		}
		stats.Updated++
	}
	return stats, nil
}

func specPtr(s ModelSpec) *ModelSpec { return &s }

// costOnly strips any price a syncer tried to state. A syncer may not set
// retail — the rule is enforced here rather than trusted.
func costOnly(rates []Rate) []Rate {
	out := make([]Rate, 0, len(rates))
	for _, r := range rates {
		r.Price = ""
		out = append(out, r)
	}
	return out
}

// mergeSpec refreshes the machine-observable facts from upstream and preserves
// the fields a human owns (MinTier, Enabled, Unavailable — the routing POLICY).
func mergeSpec(cur *ModelSpec, next ModelSpec) *ModelSpec {
	if cur == nil {
		return specPtr(next)
	}
	merged := *cur
	merged.Vendor = next.Vendor
	merged.Family = next.Family
	merged.Serves = next.Serves
	merged.Upstream = next.Upstream
	merged.Modality = next.Modality
	merged.ContextWindow = next.ContextWindow
	merged.MaxOutput = next.MaxOutput
	merged.Capabilities = next.Capabilities
	return &merged
}

// mergeRates takes the rate SET from upstream and the PRICE from whatever a
// human already set on the matching rate.
func mergeRates(cur, upstream []Rate) []Rate {
	out := make([]Rate, 0, len(upstream))
	for _, u := range upstream {
		u.Price = ""
		for _, c := range cur {
			if c.same(u) {
				u.Price = c.Price
				break
			}
		}
		out = append(out, u)
	}
	return out
}
