// Copyright © 2026 Hanzo AI. MIT License.

// A meter is one thing we charge for by quantity, and its rate.
//
// Rates used to live in hanzoai/pricing as an embedded 150KB JSON — 506 priced
// items across models, tools, infrastructure, cloud and datastore. Changing one
// number meant editing a Go module, cutting a tag, bumping the service and
// waiting for a build, so the published price and the intended price drifted for
// as long as that took. Plans already solved this: the DB is the authority, the
// embed is a seed and a loud fallback, and admin.hanzo.ai edits the rows. This
// is that same arrangement for the other half of the money.
//
// ONE TABLE FOR EVERY PRODUCT. A model bills per million tokens, a tool per
// call, compute per hour, storage per GB-month — the UNIT differs and nothing
// else does. Giving each product its own table would give each its own editor
// and its own seed, and the drift starts again in five places instead of one.
// So the unit is a value in the row, not a shape in the schema.
package rate

import (
	"context"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

// Namespace is the platform-global namespace the meter authority lives in — the
// SAME "system" namespace plans and the product catalog use. A rate is
// platform-wide and governed centrally by admin.hanzo.ai, never per tenant.
const Namespace = "system"

// AuthorityDB returns a datastore scoped to the platform meter authority. Pass
// the request context so tracing and deadlines flow; a nil context degrades to
// Background for bootstrap and CLI callers.
func AuthorityDB(ctx context.Context) *datastore.Datastore {
	if ctx == nil {
		ctx = context.Background()
	}
	return datastore.New(nscontext.WithNamespace(ctx, Namespace))
}

// IgnoreFieldMismatch lets a row written by an older binary load into a
// struct that has since gained a field, instead of failing the whole read.
var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

func init() { orm.Register[Rate]("rate") }

// Unit is what one billed quantity IS. The rate is per one of these.
// The units are string constants, not a named type: the decoder round-trips
// plain scalars, and a unit is a value the catalog already writes as a string.
const (
	// PerMTok — one million tokens, prompt+completion. Model inference.
	PerMTok = "per_mtok"
	// PerCall — one invocation. Tools, search, rerank.
	PerCall = "per_call"
	// PerHour — one machine-hour. Compute, GPU.
	PerHour = "per_hour"
	// PerGiBMonth — one GiB held for a month. Storage, datastore.
	PerGiBMonth = "per_gib_month"
	// PerGiB — one GiB moved. Egress.
	PerGiB = "per_gib"
)

// Rate is one priced quantity.
type Rate struct {
	mixin.Model[Rate]

	// Slug is the row's identity: "<product>/<key>". It is ONE field because
	// that is what the authority is queried by, the same way a plan is found by
	// its slug — a rate is looked up on every reconcile and every price read, and
	// a single indexed field is the lookup those paths already know how to do.
	//
	// Product and Rate are kept beside it as the parts, because an editor groups
	// by product and a report sums by it. The parts are derived INTO the slug by
	// Bind, never typed twice.
	Slug string `json:"slug"`

	// Product is the surface this meter belongs to: "ai", "tools", "compute",
	// "storage", "datastore", "cloud". It is what groups the editor, and what a
	// per-product report sums over.
	Product string `json:"product"`

	// Name identifies the metered thing INSIDE its product — a model name, a tool
	// name, a machine size. (Product, Rate) is the identity of a rate.
	//
	// It is NOT called Key: mixin.Entity requires a Key() METHOD, and a field of
	// that name shadows it. The interface assertion then fails silently, Query()
	// leaves its entity nil, and Get() nil-panics somewhere that names neither.
	Meter string `json:"meter"`

	// Label is what a person reads. Rate is what a system matches.
	Label string `json:"label,omitempty"`

	// Unit is what one billed quantity is. The rate below is per one of these.
	Unit string `json:"unit"`

	// Rate is the price of one Unit, in NANO-dollars — a billionth of a dollar.
	//
	// Cents cannot hold these. A cheap model is fractions of a cent per million
	// tokens, and rounding that to a cent either prices it at zero or at 100x.
	// The ledger already carries cost_nano/billed_nano/margin_nano for the same
	// reason, so this is the estate's existing precision, not a new one.
	Rate int64 `json:"rate"`

	// Currency the rate is denominated in. USD unless a row says otherwise.
	Currency string `json:"currency,omitempty"`

	// Source names who supplies the metered thing — "zen-gateway", "openrouter",
	// a provider slug. It is how a margin report joins a rate to what it cost us.
	Source string `json:"source,omitempty"`

	// Included is how much of this meter a plan grants before the rate applies.
	// -1 means unlimited; 0 means nothing is included and every unit is charged.
	// It lives on the PLAN when it varies by tier; here it is the default.
	Included int64 `json:"included,omitempty"`

	// AdminEdited marks a row a person changed through admin.hanzo.ai. The seed
	// leaves these alone — an operator's price outranks the file it came from,
	// which is the whole point of the authority.
	AdminEdited bool `json:"adminEdited,omitempty"`

	// Status is "active" or "archived". Archiving stops a meter being offered
	// without deleting the row, so historical charges still resolve their rate.
	Status string `json:"status,omitempty"`
}

// Listed reports whether this meter is currently sold. An archived row still
// resolves for a past charge; it is simply not offered.
func (m *Rate) Listed() bool { return !strings.EqualFold(m.Status, "archived") }

// Bind derives the slug from the parts. Called before any write, so the two can
// never disagree — a slug typed independently of its product and key is a third
// place for the identity to be wrong.
//
// The parts are the identity, not the key alone: the same model name can be
// metered by two products at two rates, and keying on the name would let one
// product's price overwrite another's.
func (m *Rate) Bind() { m.Slug = m.Product + "/" + m.Meter }

// Take copies what a rate SAYS onto this one — the parts, the words and the
// price — and rebinds the identity from the parts it just took.
//
// It is the ONE definition of "what a write may set", so the seed reconciling a
// published row and an admin saving an edited one move exactly the same fields.
// They were two functions before, in two packages, listing the same eight fields
// in the same order; nothing made them agree, and the seed's own comparator had
// already drifted from its copy once (four window fields on the plan model, the
// same shape of bug, which reached production).
//
// What it deliberately does NOT take is the bookkeeping: the datastore binding,
// the timestamps, and AdminEdited. Those belong to the caller, because whether a
// write is a person's decision is a fact about the REQUEST, not about the values
// it carried.
func (m *Rate) Take(src *Rate) {
	m.Product = src.Product
	m.Meter = src.Meter
	m.Bind()
	m.Label = src.Label
	m.Unit = src.Unit
	m.Rate = src.Rate
	m.Currency = src.Currency
	m.Source = src.Source
	m.Included = src.Included
	// Only when it says one: an import that omits status must not silently
	// unlist a row that an operator archived.
	if src.Status != "" {
		m.Status = src.Status
	}
}

// New returns an empty meter bound to db.
func New(db *datastore.Datastore) *Rate {
	m := new(Rate)
	m.Init(db)
	return m
}

// Query returns a meter query against db.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("rate") }
