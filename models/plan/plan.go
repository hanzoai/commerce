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

	// Managed marks a row OWNED by the seed or the SuperAdmin CRUD (authoritative).
	// The corrective boot seed force-corrects UNMANAGED rows — accidental partials
	// written by a subscription-flow path (Red: the bundle expansion wrote a
	// Price=0, envelope-less row that under-charged a direct sub) — to the embed,
	// and LEAVES managed rows so an admin price edit is preserved. One flag
	// distinguishes "authoritative" from "accidental", closing the divergence.
	Managed bool `json:"managed,omitempty"`

	// Status is the row's lifecycle, on the Shopify product model: a plan is
	// ACTIVE (sold and listed), DRAFT (exists, not yet listed) or ARCHIVED
	// (retired, kept). Only ACTIVE rows reach the public catalog.
	//
	// It exists because without it "stop selling this" and "destroy this" were the
	// SAME operation. The public read returned every stored row, so the only way to
	// unlist a plan was DELETE — which takes the row's history with it and orphans
	// any subscription that recorded the slug. Retiring a tier should never be
	// destructive; archiving keeps the row resolvable for invoices and renewals
	// that already reference it.
	//
	// EMPTY MEANS ACTIVE, deliberately. Every row written before this field existed
	// has no value for it, and a zero-value that meant "draft" would unlist the
	// entire catalog on deploy. Use Listed() rather than comparing this directly.
	Status string `json:"status,omitempty"`

	// Metadata is the display envelope (features/limits/bundles/includedIn) — the
	// typed money fields above are authoritative, this is presentation. Metadata_
	// MUST be datastore:",noindex" (persisted), NOT "-" (skipped): with "-" the
	// whole envelope Save()s but never round-trips, so a DB read drops
	// limits/minSeats/features (prod: team.minSeats served null). Mirrors
	// catalogentry exactly — the persisted-blob pattern is the one way.
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	Ref refs.EcommerceRef `json:"ref,omitempty"`
}

// Plan lifecycle, on the Shopify product model. A plan is never deleted to stop
// selling it — it is archived, so invoices and subscriptions that recorded the
// slug still resolve.
const (
	StatusActive   = "active"   // sold and listed publicly
	StatusDraft    = "draft"    // exists, not listed — being written
	StatusArchived = "archived" // retired, kept for history
)

// Listed reports whether this plan is ON SALE — the ONE predicate behind both
// halves of that, because they are one decision:
//
//   - it appears in the public catalog (GET /v1/billing/plans), and
//   - a NEW subscription may open on it (the three purchase entrypoints).
//
// Resolving a plan is a DIFFERENT question and is deliberately not gated here:
// resolveSubscriptionPlan answers "which row is this" and must keep answering for
// an archived plan, or a renewal or invoice on a retired tier would fail to price
// itself. Retiring a tier stops new sales; it never strands a subscriber.
//
// An empty Status counts as active: every row written before Status existed has
// no value for it, and treating that as unlisted would blank the catalog the
// moment this ships. Only an explicit draft/archived hides a row.
func (p *Plan) Listed() bool {
	return p.Status == "" || p.Status == StatusActive
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

// Seed reconciles the plan authority to the given embed rows. It is safe to run
// on EVERY boot (cheap: one point query per row; writes only when needed) and is
// the ONE seeder — the count-gated variant is gone, because a count gate makes a
// fixed re-seed a no-op that keeps serving bad rows.
//
// Per embed row:
//   - MISSING          → create it, Managed=true.
//   - present, UNMANAGED → FORCE-CORRECT its typed money fields + envelope to the
//     embed and mark Managed=true. An unmanaged row was written by a
//     subscription-flow path (bundle expansion / lazy resolve) and may be partial
//     or wrong (Red: bundle wrote Price=0) — this is the corrective repair.
//   - present, MANAGED  → LEAVE. Seed/admin already own it, so an admin price edit
//     is preserved (the whole point of the editable authority).
//
// Idempotent: after the first corrective pass every row is Managed, so later runs
// write nothing. seedMu makes the per-slug check-then-write atomic in-process.
// Returns (created, corrected).
func Seed(db *datastore.Datastore, rows []*Plan) (created, corrected int, err error) {
	seedMu.Lock()
	defer seedMu.Unlock()
	for _, r := range rows {
		if r == nil || r.Slug == "" {
			continue
		}
		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", r.Slug).Get()
		if qerr != nil {
			return created, corrected, qerr
		}
		if ok {
			if existing.Managed {
				continue
			}
			copyInto(existing, r)
			existing.Managed = true
			if uerr := existing.Update(); uerr != nil {
				return created, corrected, uerr
			}
			corrected++
			continue
		}
		e := New(db)
		copyInto(e, r)
		e.Managed = true
		if cerr := e.Create(); cerr != nil {
			return created, corrected, cerr
		}
		created++
	}
	return created, corrected, nil
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
