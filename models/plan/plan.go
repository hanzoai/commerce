package plan

import (
	"context"
	"slices"
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

	// AdminEdited marks a row whose CURRENT VALUE WAS DECIDED BY A HUMAN through
	// the SuperAdmin CRUD. It is the discriminator Managed could never be.
	//
	// Managed is set by BOTH the seed and the admin CRUD, so it carries two
	// different facts under one name and the seed cannot tell its own prior
	// output from a deliberate price edit. The consequence was that the seed
	// could never correct itself: once a row existed it was frozen forever, so
	// publishing a new catalog created the plans that were missing and left every
	// plan that had changed at its old price — a catalog half-new and half-stale,
	// which is worse than either.
	//
	// Only api/plan's Create/Update sets this. Seed reconciles anything else to
	// the published catalog, which is exactly the rule everyone assumed already
	// held: the package is the source, an admin edit is an override.
	AdminEdited bool `json:"adminEdited,omitempty"`

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

	// The DISPLAY ENVELOPE: presentation, not money. The typed columns above are
	// what charges; these are what the pricing page renders.
	//
	// They are JSON-visible and datastore-SKIPPED on purpose. JSON-visible so the
	// model's shape IS the wire shape — the admin CRUD decodes a request body
	// straight onto a Plan, so whatever GET /v1/billing/plans emits, PUT
	// /v1/plans/entries/:slug accepts. Before they existed the envelope lived ONLY
	// inside Metadata as a packed JSON string, so an admin PUT carrying `features`
	// set the price and silently discarded the copy: a tier could be repriced but
	// never re-described, which is how a plan comes to be sold at one price while
	// its own feature list quotes another.
	//
	// Datastore-skipped because they still PERSIST packed into Metadata_ (Save
	// packs, Load unpacks) — one storage location, unchanged on disk, so existing
	// rows keep working and there is no migration.
	Features   []string `json:"features,omitempty" datastore:"-"`
	Bundles    []string `json:"bundles,omitempty" datastore:"-"`
	IncludedIn []string `json:"includedIn,omitempty" datastore:"-"`
	Limits     *Limits  `json:"limits,omitempty" datastore:"-"`

	// Metadata carries anything else a plan needs to hold, plus the packed
	// envelope above. Metadata_ MUST be datastore:",noindex" (persisted), NOT "-"
	// (skipped): with "-" the whole blob Save()s but never round-trips, so a DB
	// read drops limits/minSeats/features (prod: team.minSeats served null).
	// Mirrors catalogentry exactly — the persisted-blob pattern is the one way.
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

// Limits is a plan's published allowance block — rate ceilings, seat floors and
// included credit. It lives HERE, with the plan, because it is plan data: the
// catalog publishes it and GET /v1/billing/plans serves it verbatim. api/billing
// aliases this type rather than declaring a second copy.
type Limits struct {
	// Subscription (API) limits
	RequestsPerMinute *int `json:"requestsPerMinute,omitempty"`
	TokensPerMinute   *int `json:"tokensPerMinute,omitempty"`
	FreeCredit        *int `json:"freeCredit,omitempty"`
	MaxMembers        *int `json:"maxMembers,omitempty"`

	// MinSeats is the minimum billable seat count for per-seat plans
	// (price_ref.recurring.per_seat) — the ONE canonical home for seat minimums.
	// Billing charges at least this many seats.
	MinSeats *int `json:"minSeats,omitempty"`

	// TeamGuests is the back-compat source for the team.guests entitlement:
	// max invited guests on the holder's hanzo.team workspace (-1 = unlimited).
	TeamGuests *int `json:"teamGuests,omitempty"`

	// IncludedCreditUsd is a legacy alias (entitlements["commerce.included_credit_usd"]).
	// IncludedCloudCredits(+PerUser) is the cloud allowance the monthly allotment
	// grants (the canonical cloud.included_credits_usd entitlement).
	IncludedCreditUsd           *int `json:"includedCreditUsd,omitempty"`
	IncludedCloudCredits        *int `json:"includedCloudCredits,omitempty"`
	IncludedCloudCreditsPerUser *int `json:"includedCloudCreditsPerUser,omitempty"`

	// DNS limits
	Zones          *int `json:"zones,omitempty"`
	RecordsPerZone *int `json:"recordsPerZone,omitempty"`
	QueriesPerDay  *int `json:"queriesPerDay,omitempty"`
}

// EnvelopeKey is the ONE Metadata key the display envelope is packed under. One
// key holding one JSON string, so it round-trips cleanly through the
// Map[string]interface{} the ORM deserializes Metadata into — a string survives,
// where a nested struct would degrade to map[string]interface{} and lose the
// typed Limits shape.
const EnvelopeKey = "envelope"

// envelope is the on-disk form of the display fields. It is unexported because
// nothing outside this file should construct one: the typed fields on Plan are
// the interface, and this is only how they are written down.
type envelope struct {
	Features   []string `json:"features,omitempty"`
	Bundles    []string `json:"bundles,omitempty"`
	IncludedIn []string `json:"includedIn,omitempty"`
	Limits     *Limits  `json:"limits,omitempty"`
}

func (p *Plan) Load(ps []datastore.Property) (err error) {
	// Load supported properties
	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		if err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata); err != nil {
			return err
		}
	}

	// Unpack the display envelope onto its typed fields. A row stores it as one
	// JSON string under EnvelopeKey; callers read Features/Limits/… directly.
	if s, ok := p.Metadata[EnvelopeKey].(string); ok && s != "" {
		var env envelope
		if err := json.DecodeBytes([]byte(s), &env); err == nil {
			p.Features, p.Bundles, p.IncludedIn, p.Limits = env.Features, env.Bundles, env.IncludedIn, env.Limits
		}
	}

	return nil
}

func (p *Plan) Save() (ps []datastore.Property, err error) {
	// Pack the display envelope into Metadata under its one key.
	//
	// It goes into Metadata, not Metadata_, because Metadata is what actually
	// persists: the ORM stores a row as JSON of the struct, and Metadata_ is
	// `json:"-"`. Metadata_ is the older datastore-property path, kept because
	// Load still honors it, but a value written only there does not survive.
	env := envelope{Features: p.Features, Bundles: p.Bundles, IncludedIn: p.IncludedIn, Limits: p.Limits}
	if len(env.Features) > 0 || len(env.Bundles) > 0 || len(env.IncludedIn) > 0 || env.Limits != nil {
		if p.Metadata == nil {
			p.Metadata = Map{}
		}
		p.Metadata[EnvelopeKey] = string(json.EncodeBytes(&env))
	} else if p.Metadata != nil {
		// Nothing to carry: drop a stale envelope rather than leave the previous
		// one behind, or clearing a plan's features would be a no-op.
		delete(p.Metadata, EnvelopeKey)
	}
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

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

// Seed reconciles the plan authority to the published catalog. It is safe to run
// on EVERY boot (cheap: one point query per row; writes only when needed) and is
// the ONE seeder.
//
// The rule is simply: the published catalog decides, an admin edit overrides.
//
//   - MISSING            → create it.
//   - present, ADMIN-EDITED → LEAVE. A human decided this value; the catalog does
//     not get to overwrite it. That is the whole point of an editable authority.
//   - present, otherwise → RECONCILE its typed money fields + display envelope to
//     the catalog. This covers both a row the seed itself wrote earlier (which it
//     previously could never correct — see AdminEdited) and a partial row written
//     by a subscription-flow path such as the bundle expansion, which once wrote
//     Price=0 and under-charged.
//
// Rows the catalog NO LONGER publishes are ARCHIVED, not deleted and not left
// listed. A tier that stops being published must stop being sold, or retiring it
// requires a second manual step that is easy to forget and easy to get wrong;
// archiving keeps the row resolvable for invoices and renewals that recorded the
// slug. An admin-edited row is never archived this way — a plan a human added on
// purpose is not "missing from the catalog", it is theirs.
//
// Idempotent: a second run finds every row already equal to the catalog and
// writes nothing. seedMu makes the per-slug check-then-write atomic in-process.
// Returns (created, corrected) where corrected counts reconciles + archives.
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
			if existing.AdminEdited {
				continue
			}
			if planEqual(existing, r) {
				continue // already the published value — write nothing
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

	// Retire what the catalog stopped publishing.
	published := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r != nil {
			published[r.Slug] = true
		}
	}
	var all []*Plan
	if _, qerr := Query(db).GetAll(&all); qerr != nil {
		return created, corrected, qerr
	}
	for _, row := range all {
		if row.AdminEdited || published[row.Slug] || !row.Listed() {
			continue
		}
		// Re-load through the same point query the reconcile above uses. A row
		// that arrived via GetAll is a VALUE — it carries no datastore binding,
		// so Update on it writes nowhere and reports no error.
		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", row.Slug).Get()
		if qerr != nil {
			return created, corrected, qerr
		}
		if !ok {
			continue
		}
		existing.Status = StatusArchived
		if uerr := existing.Update(); uerr != nil {
			return created, corrected, uerr
		}
		corrected++
	}
	return created, corrected, nil
}

// planEqual reports whether a stored row already carries the catalog's values, so
// a reconciling boot writes nothing when there is nothing to change. Compares the
// fields copyInto actually sets — money, identity and the display envelope.
func planEqual(a, b *Plan) bool {
	if a.Name != b.Name || a.Description != b.Description || a.Category != b.Category ||
		a.Price != b.Price || a.PriceAnnual != b.PriceAnnual || a.Currency != b.Currency ||
		a.Interval != b.Interval || a.IntervalCount != b.IntervalCount ||
		a.TrialPeriodDays != b.TrialPeriodDays || a.PerSeat != b.PerSeat ||
		a.ContactSales != b.ContactSales || a.Popular != b.Popular {
		return false
	}
	return slices.Equal(a.Features, b.Features) &&
		slices.Equal(a.Bundles, b.Bundles) &&
		slices.Equal(a.IncludedIn, b.IncludedIn) &&
		limitsEqual(a.Limits, b.Limits)
}

func limitsEqual(a, b *Limits) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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
	dst.Features = src.Features
	dst.Bundles = src.Bundles
	dst.IncludedIn = src.IncludedIn
	dst.Limits = src.Limits
	dst.Metadata = src.Metadata
}
