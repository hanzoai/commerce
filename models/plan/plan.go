package plan

import (
	"context"
	"sync"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/refs"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

// seedMu serializes concurrent Seed calls IN-PROCESS so the per-slug
// check-then-create is atomic (Red F3: RunInTransaction is a no-op, plan rows are
// hashid-keyed so two racing creates can't ON-CONFLICT-collapse). Commerce's data
// PVC is ReadWriteOnce (single writer) and the boot seed runs BEFORE the HTTP
// server accepts requests, so there is no cross-process concurrent writer; this
// mutex closes the only real window — two concurrent in-process callers (e.g. the
// admin POST /plans/seed racing itself). Keeping plan hashid-keyed (not
// WithStringKey) avoids changing sub.PlanId across the money path.
var seedMu sync.Mutex

// Namespace is the platform-global namespace the plan authority lives in — the
// SAME "system" namespace the product catalog uses. Subscription and DNS plan
// pricing is platform-wide and governed centrally by admin.hanzo.ai, NOT per
// tenant, so every authoritative touch of a plan (the boot seed, the SuperAdmin
// CRUD, GET /v1/billing/plans, and resolveSubscriptionPlan → the internal-ledger
// charge) resolves HERE. Kept identical to catalog's "system" so one platform
// namespace holds all cross-tenant CMS/pricing data.
const Namespace = "system"

// AuthorityDB returns a datastore scoped (via context — the only namespacing the
// SQL layer honors) to the platform plan-authority namespace. Pass the request
// context so tracing/deadlines flow through; a nil context degrades to
// Background (bootstrap/CLI callers).
func AuthorityDB(ctx context.Context) *datastore.Datastore {
	if ctx == nil {
		ctx = context.Background()
	}
	return datastore.New(nscontext.WithNamespace(ctx, Namespace))
}

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

func init() { orm.Register[Plan]("plan") }

// Based On Stripe Plan
// Stripe\Plan JSON: {
//   "id": "gold21323",
//   "object": "plan",
//   "amount": 2000,
//   "created": 1386247539,
//   "currency": "usd",
//   "interval": "month",
//   "interval_count": 1,
//   "livemode": false,
//   "metadata": {
//   },
//   "name": "New plan name",
//   "statement_descriptor": null,
//   "trial_period_days": null
// }

type Plan struct {
	mixin.Model[Plan]

	// Unique human readable id
	Slug string `json:"slug"`
	// Internal id
	SKU string `json:"sku"`

	// Human readable name
	Name        string `json:"name"`
	Description string `json:"description"`

	// Category is the plan family ("personal"/"team"/"enterprise"/"world"/
	// "social"/"dns"); GET /v1/billing/plans?category= filters on it.
	Category string `json:"category"`

	Price currency.Cents `json:"price"`
	// PriceAnnual is the per-month price when billed annually, in cents — the
	// authoritative annual price (previously derived at the read edge from the
	// embed). Like Price it is a TYPED money field, never an untyped Metadata
	// value, so a stored/spoofed plan copy can never inflate it.
	PriceAnnual     currency.Cents `json:"priceAnnual"`
	Currency        currency.Type  `json:"currency"`
	Interval        Interval       `json:"interval"`
	IntervalCount   int            `json:"intervalCount"`
	TrialPeriodDays int            `json:"trialPeriodDays"`

	// PerSeat marks a plan billed per seat (catalog price_ref.recurring.per_seat):
	// invoices charge Price × subscription quantity, floored at 1.
	PerSeat bool `json:"perSeat"`

	// ContactSales marks a custom / "contact sales" plan whose price is NULL, not
	// $0 — the ONE way to preserve the free($0)-vs-custom(null) distinction while
	// keeping Price a typed, non-nullable Cents (mirrors the embed's staticPlan).
	// A null-priced plan is stored Price=0 + ContactSales=true; a free plan is
	// Price=0 + ContactSales=false. Never coerce null→0 WITHOUT this flag, or a
	// custom plan spuriously reads as a $0 charge.
	ContactSales bool `json:"contactSales,omitempty"`
	// Popular flags the highlighted tier within a category (display only).
	Popular bool `json:"popular,omitempty"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`

	Ref refs.EcommerceRef `json:"ref,omitempty"`
}

func (p *Plan) Load(ps []datastore.Property) (err error) {
	// Load supported properties
	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Plan) Save() (ps []datastore.Property, err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	if err != nil {
		return nil, err
	}

	// Save properties
	return datastore.SaveStruct(p)
}

func (p *Plan) Validator() *val.Validator {
	return val.New()
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("plan")
}

// Seed upserts the given plan rows into db (which MUST be namespaced to the
// plan-authority Namespace). IDEMPOTENT + NON-DESTRUCTIVE: a plan whose slug
// already exists is left UNTOUCHED, so an admin edit is never clobbered; only
// missing slugs are created. The source is injected (the embed lives in
// api/billing) so the write mechanism stays decoupled from the catalog source.
// Returns the number created.
func Seed(db *datastore.Datastore, rows []*Plan) (created int, err error) {
	seedMu.Lock()
	defer seedMu.Unlock()
	for _, r := range rows {
		if r == nil || r.Slug == "" {
			continue
		}
		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", r.Slug).Get()
		if qerr != nil {
			return created, qerr
		}
		if ok {
			continue
		}
		e := New(db)
		copyInto(e, r)
		if err := e.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// SeedIfEmpty seeds rows only when NONE of the categories they cover are present
// yet — a cheap per-category count gates the full per-row create, so it is safe
// to call on every bootstrap (mirrors catalogentry.SeedInfraIfEmpty). Once any
// plan in a seeded category exists (seeded, admin-edited, or admin-deleted) the
// gate stays shut, so admin state — including deletions of individual plans —
// stays authoritative. A total wipe of every seeded category re-opens the gate;
// Seed then only re-creates the missing slugs (still non-destructive).
func SeedIfEmpty(db *datastore.Datastore, rows []*Plan) (created int, err error) {
	cats := map[string]bool{}
	for _, r := range rows {
		if r != nil {
			cats[r.Category] = true
		}
	}
	total := 0
	for cat := range cats {
		n, cerr := Query(db).Filter("Category=", cat).Count()
		if cerr != nil {
			return 0, cerr
		}
		total += n
	}
	if total > 0 {
		return 0, nil
	}
	return Seed(db, rows)
}

// copyInto copies the seed fields from src onto the fresh, db-bound dst so the
// created row keeps dst's datastore binding while taking src's values. The
// authoritative money fields are the typed columns (Price/PriceAnnual/…); the
// rich display envelope rides Metadata.
func copyInto(dst, src *Plan) {
	dst.Slug = src.Slug
	dst.SKU = src.SKU
	dst.Name = src.Name
	dst.Description = src.Description
	dst.Category = src.Category
	dst.Price = src.Price
	dst.PriceAnnual = src.PriceAnnual
	dst.Currency = src.Currency
	dst.Interval = src.Interval
	dst.IntervalCount = src.IntervalCount
	dst.TrialPeriodDays = src.TrialPeriodDays
	dst.PerSeat = src.PerSeat
	dst.ContactSales = src.ContactSales
	dst.Popular = src.Popular
	dst.Metadata = src.Metadata
}
