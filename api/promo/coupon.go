// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package promo

// Plan coupons — a CODE a buyer types at checkout for a percent off the FIRST
// period of one plan purchase. They are the per-purchase companion of the
// platform promo above and live beside it, in the reserved platform namespace:
//
//   - The plan catalog is the platform's, not any tenant's, so the store that
//     sells a plan is the platform itself.
//   - "admin" is the org whose membership IS the SuperAdmin predicate, so the
//     org-scoped coupon CRUD (/v1/coupon, which writes into the caller's own
//     namespace) can only reach these rows for a SuperAdmin. A brand org or the
//     "system" catalog namespace would be writable by whoever administers an
//     org of that name.
//   - The sale and the public plans quote read one place, as the promo does.
//
// A plan coupon is a models/coupon row with Filter == coupon.PlanFilter and
// Type == percent; nothing else in the namespace is honoured as one. It covers
// exactly the plan slugs it names — a coupon that names none covers nothing — and
// SetCoupon refuses a slug that is not a paid plan on sale. Each use
// is a models/couponredemption row keyed by (code, org) and by (code, card), so
// an org and a card each redeem a code once, and Used counts uses against
// Limit under the code's lock, so the limit cannot be raced past.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/couponredemption"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/timeutil"

	. "github.com/hanzoai/commerce/types"
)

// Why a coupon cannot be used, as the plans quote names it (couponError). A
// coupon that is disabled or not yet started answers unknown, so a campaign
// cannot be discovered before it opens.
const (
	CouponUnknown       = "unknown"
	CouponExpired       = "expired"
	CouponExhausted     = "exhausted"
	CouponNotApplicable = "not_applicable"
)

// maxCouponCode bounds a code before it reaches a query.
const maxCouponCode = 64

// CouponCode is a code as it is stored and compared: trimmed and upper-cased,
// so codes are case-insensitive. A code that could not have been stored —
// empty, longer than maxCouponCode, or holding anything but A-Z, 0-9, '-' and
// '_' — is "".
func CouponCode(raw string) string {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if c == "" || len(c) > maxCouponCode {
		return ""
	}
	for _, r := range c {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return ""
		}
	}
	return c
}

// Offer is a plan coupon that may be redeemed now.
type Offer struct {
	// Code is the canonical code (CouponCode).
	Code string
	// Percent is the whole percent off the first period, 1..100.
	Percent int
	// End is when the coupon stops being redeemable, nil when it never does.
	End *time.Time
	// plans is every slug the coupon names (Plans and ProductId).
	plans []string
}

// AppliesTo reports whether the coupon covers a plan slug: only a slug it NAMES.
// A coupon that names none covers nothing, so a row written without a plan list
// can never discount the whole catalog. Whether the plan charges at all is the
// caller's question too: a free plan has nothing to take a percent of.
func (o *Offer) AppliesTo(slug string) bool {
	for _, s := range o.plans {
		if s == slug {
			return true
		}
	}
	return false
}

// Label is how the coupon reads on the invoice it discounts.
func (o *Offer) Label() string {
	return engine.Coupon{Code: o.Code, Percent: o.Percent}.Label()
}

// FindCoupon resolves raw to an Offer redeemable at now, or names why not
// (CouponUnknown, CouponExpired, CouponExhausted). It only reads.
func FindCoupon(ctx context.Context, raw string, now time.Time) (*Offer, string) {
	code := CouponCode(raw)
	if code == "" {
		return nil, CouponUnknown
	}
	cpn, _, err := loadCoupon(platform(ctx), code)
	if err != nil {
		log.Error("plan coupon %s could not be read: %v", code, err)
		return nil, CouponUnknown
	}
	if cpn == nil {
		return nil, CouponUnknown
	}
	if reason := standing(cpn, now); reason != "" {
		return nil, reason
	}
	return offerOf(cpn), ""
}

// standing is the one rule a stored coupon is judged by, at now: a plan coupon,
// percent-off within 1..100, enabled, inside [StartDate, EndDate], and not used
// up. "" means it stands.
func standing(c *coupon.Coupon, now time.Time) string {
	switch {
	case c.Filter != coupon.PlanFilter, c.Type != coupon.Percent, !c.Enabled,
		c.Amount < 1, c.Amount > 100:
		return CouponUnknown
	case !timeutil.IsZero(c.StartDate) && now.Before(c.StartDate):
		return CouponUnknown
	case !timeutil.IsZero(c.EndDate) && now.After(c.EndDate):
		return CouponExpired
	case c.Limit > 0 && c.Used >= c.Limit:
		return CouponExhausted
	}
	return ""
}

func offerOf(c *coupon.Coupon) *Offer {
	o := &Offer{Code: c.Code_, Percent: c.Amount}
	if !timeutil.IsZero(c.EndDate) {
		end := c.EndDate.UTC()
		o.End = &end
	}
	o.plans = append(o.plans, c.Plans...)
	if p := strings.TrimSpace(c.ProductId); p != "" {
		o.plans = append(o.plans, p)
	}
	return o
}

// loadCoupon reads the ONE plan-coupon row stored under code, bound for
// writing, with its key; (nil, nil, nil) when there is none.
//
// It finds the key by query and loads by key on purpose. A ModelQuery.Get
// re-runs Init after the load, and Init applies the model's tag defaults to
// every zero field — Enabled is `default:true`, so a coupon a SuperAdmin
// switched off would read back on. Loading by key applies the defaults to the
// empty struct first and lets the stored row overwrite them.
//
// Two rows under one code is refused rather than resolved by whichever the
// query returned first: which percent a buyer gets must not be an accident.
func loadCoupon(db *datastore.Datastore, code string) (*coupon.Coupon, datastore.Key, error) {
	rows := make([]*coupon.Coupon, 0, 2)
	keys, err := coupon.Query(db).Filter("Code_=", code).Limit(2).GetAll(&rows)
	if err != nil {
		return nil, nil, err
	}
	switch len(keys) {
	case 0:
		return nil, nil, nil
	case 1:
	default:
		return nil, nil, fmt.Errorf("%d coupons are stored under code %s", len(keys), code)
	}
	cpn := coupon.New(db)
	if err := cpn.Get(keys[0]); err != nil {
		return nil, nil, err
	}
	return cpn, keys[0], nil
}

// couponLocks serializes every write to one code's rows — a reservation, its
// release, and a SuperAdmin edit — so Used is a read-modify-write nobody
// interleaves. Commerce runs as one process per store (replicas 1, Recreate, a
// ReadWriteOnce volume), so an in-process lock is the serialization point, as
// it is for api/coupon's redeemLocks and the gift-card ledger.
var couponLocks sync.Map // map[string]*sync.Mutex

func couponLock(code string) *sync.Mutex {
	m, _ := couponLocks.LoadOrStore(code, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// A reservation refused because the org, or the card, has already redeemed the
// code. Each org and each card redeems a code once.
var (
	ErrCouponRedeemedByOrg  = errors.New("this organization has already redeemed this coupon")
	ErrCouponRedeemedByCard = errors.New("this card has already redeemed this coupon")
)

// CouponRefusal is a reservation refused because the coupon no longer stands:
// Reason is CouponUnknown, CouponExpired or CouponExhausted.
type CouponRefusal struct{ Reason string }

func (e CouponRefusal) Error() string { return "coupon " + e.Reason }

// Hold is one reserved use of a coupon: its redemption rows written and Used
// counted. Release gives the use back when the sale it was reserved for did
// not take the money.
type Hold struct {
	ctx      context.Context
	code     string
	org      string
	cardID   string
	orgID    string
	released bool
}

// Redeemed reports whether org has already redeemed code. It is the early,
// read-only answer, so a buyer is told before their card is vaulted; Reserve
// asks again under the lock and is the one that binds, which is also why a read
// that fails answers false here — Reserve fails closed on the same failure.
func Redeemed(ctx context.Context, raw, org string) bool {
	code := CouponCode(raw)
	if code == "" {
		return false
	}
	ok, _ := redemptionExists(platform(ctx), couponredemption.DeterministicID(code, orgRedeemer(org)))
	return ok
}

// Reserve takes one use of code for org paying with instrument, before any
// money moves. Under the code's lock it re-reads the coupon and refuses one
// that no longer stands (CouponRefusal), refuses an org or a card that has
// redeemed the code already (ErrCouponRedeemedByOrg, ErrCouponRedeemedByCard), and
// otherwise writes both redemption rows and counts the use. instrument is the
// card's stable identity and is required: a use with no instrument cannot be
// held to the one-per-card rule.
func Reserve(ctx context.Context, raw, org, instrument string, now time.Time) (*Hold, error) {
	code := CouponCode(raw)
	org = strings.TrimSpace(org)
	instrument = strings.TrimSpace(instrument)
	if code == "" {
		return nil, CouponRefusal{CouponUnknown}
	}
	if org == "" || instrument == "" {
		return nil, errors.New("coupon: a use needs an org and a card")
	}

	mu := couponLock(code)
	mu.Lock()
	defer mu.Unlock()

	db := platform(ctx)
	cpn, _, err := loadCoupon(db, code)
	if err != nil {
		return nil, err
	}
	if cpn == nil {
		return nil, CouponRefusal{CouponUnknown}
	}
	if reason := standing(cpn, now); reason != "" {
		return nil, CouponRefusal{reason}
	}

	orgID := couponredemption.DeterministicID(code, orgRedeemer(org))
	card := cardRedeemer(instrument)
	cardID := couponredemption.DeterministicID(code, card)
	if ok, err := redemptionExists(db, orgID); err != nil {
		return nil, err
	} else if ok {
		return nil, ErrCouponRedeemedByOrg
	}
	if ok, err := redemptionExists(db, cardID); err != nil {
		return nil, err
	} else if ok {
		return nil, ErrCouponRedeemedByCard
	}

	// The hold outlives nothing but this sale, and its release must still run
	// when the buyer has hung up, so it keeps the context's values and drops its
	// cancellation.
	h := &Hold{ctx: context.WithoutCancel(ctx), code: code, org: org, orgID: orgID, cardID: cardID}
	if err := putRedemption(db, orgID, code, orgRedeemer(org)); err != nil {
		return nil, err
	}
	if err := putRedemption(db, cardID, code, card); err != nil {
		h.undo(db)
		return nil, err
	}
	cpn.Used++
	if err := cpn.Update(); err != nil {
		h.undo(db)
		return nil, err
	}
	return h, nil
}

// Release gives a reserved use back: the redemption rows are removed and Used
// counts one fewer. It is for a sale that took no money — a declined card — and
// is a no-op the second time.
func (h *Hold) Release() {
	if h == nil || h.released {
		return
	}
	h.released = true
	mu := couponLock(h.code)
	mu.Lock()
	defer mu.Unlock()

	db := platform(h.ctx)
	h.undo(db)
	cpn, _, err := loadCoupon(db, h.code)
	if err != nil || cpn == nil {
		log.Error("plan coupon %s: a released use was not uncounted (org %s): %v", h.code, h.org, err)
		return
	}
	if cpn.Used > 0 {
		cpn.Used--
	}
	if err := cpn.Update(); err != nil {
		log.Error("plan coupon %s: a released use was not uncounted (org %s): %v", h.code, h.org, err)
	}
}

// undo removes the hold's redemption rows. A row that cannot be removed leaves
// that org or card unable to redeem the code again — the safe direction.
func (h *Hold) undo(db *datastore.Datastore) {
	for _, id := range []string{h.orgID, h.cardID} {
		r := couponredemption.New(db)
		if err := r.Get(db.NewKey(r.Kind(), id, 0, nil)); err != nil {
			continue
		}
		if err := r.Delete(); err != nil {
			log.Error("plan coupon %s: redemption %s was not removed: %v", h.code, id, err)
		}
	}
}

func orgRedeemer(org string) string { return "org:" + strings.ToLower(org) }

// cardRedeemer is a card's identity as a redemption row stores it: hashed, so
// the platform namespace holds no card facts.
func cardRedeemer(instrument string) string {
	sum := sha256.Sum256([]byte(instrument))
	return "card:" + hex.EncodeToString(sum[:16])
}

func redemptionExists(db *datastore.Datastore, id string) (bool, error) {
	r := couponredemption.New(db)
	err := r.Get(db.NewKey(r.Kind(), id, 0, nil))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return false, nil
	}
	return false, err
}

func putRedemption(db *datastore.Datastore, id, code, redeemer string) error {
	r := couponredemption.New(db)
	r.SetId(id)
	r.CouponCode = code
	r.UserId = redeemer
	return r.Create()
}

// ---- SuperAdmin surface ---------------------------------------------------

// CouponSpec is a SuperAdmin's whole statement of one plan coupon, the body of
// PUT /v1/platform/coupons/{code}: percent off the first period (1..100), the
// UTC window it is redeemable in (either end open when absent), how many uses
// it has in all (0 = no limit), the plan slugs it covers (required: at least
// one, each a paid plan on sale), and whether it is on.
type CouponSpec struct {
	Percent int        `json:"percent"`
	Start   *time.Time `json:"start,omitempty"`
	End     *time.Time `json:"end,omitempty"`
	Limit   int        `json:"limit"`
	Plans   []string   `json:"plans"`
	Enabled bool       `json:"enabled"`
}

// CouponView is a plan coupon read back: its code, its spec and how many uses
// it has had.
type CouponView struct {
	Code string `json:"code"`
	CouponSpec
	Used int `json:"used"`
}

func viewOf(c *coupon.Coupon) CouponView {
	v := CouponView{
		Code: c.Code_,
		CouponSpec: CouponSpec{
			Percent: c.Amount,
			Limit:   c.Limit,
			Plans:   append([]string{}, c.Plans...),
			Enabled: c.Enabled,
		},
		Used: c.Used,
	}
	if !timeutil.IsZero(c.StartDate) {
		s := c.StartDate.UTC()
		v.Start = &s
	}
	if !timeutil.IsZero(c.EndDate) {
		e := c.EndDate.UTC()
		v.End = &e
	}
	return v
}

func (v CouponView) audit(by string) Map {
	return Map{
		"code": v.Code, "percent": v.Percent, "start": v.Start, "end": v.End,
		"limit": v.Limit, "plans": v.Plans, "enabled": v.Enabled, "used": v.Used, "by": by,
	}
}

// errCouponSpec is a spec the coupon cannot be stored as.
type errCouponSpec struct{ msg string }

func (e errCouponSpec) Error() string { return e.msg }

// IsCouponSpecRefused reports whether SetCoupon refused the request itself — a
// bad code, percent, window, limit or plan list — rather than failing to store it.
func IsCouponSpecRefused(err error) bool {
	var e errCouponSpec
	return errors.As(err, &e)
}

// ErrCouponNotFound is ReadCoupon of a code with no plan coupon.
var ErrCouponNotFound = errors.New("coupon not found")

// ReadCoupon returns the plan coupon stored under code.
func ReadCoupon(ctx context.Context, raw string) (*CouponView, error) {
	code := CouponCode(raw)
	if code == "" {
		return nil, ErrCouponNotFound
	}
	cpn, _, err := loadCoupon(platform(ctx), code)
	if err != nil {
		return nil, err
	}
	if cpn == nil || cpn.Filter != coupon.PlanFilter {
		return nil, ErrCouponNotFound
	}
	v := viewOf(cpn)
	return &v, nil
}

// SetCoupon creates or replaces the plan coupon stored under code with spec, and
// records the change, with who made it (by), as a coupon.created or
// coupon.updated billing event in the platform namespace — the audit record. Used
// is never taken from the spec: it counts real uses.
//
// The audit row is written after the coupon. If it cannot be, SetCoupon fails
// even though the coupon was stored, so a change nobody can trace is never
// reported as done; the same PUT again stores the same coupon and records it.
func SetCoupon(ctx context.Context, raw string, spec CouponSpec, by string) (*CouponView, error) {
	code := CouponCode(raw)
	switch {
	case code == "":
		return nil, errCouponSpec{fmt.Sprintf("code must be 1-%d of A-Z, 0-9, '-' and '_'", maxCouponCode)}
	case spec.Percent < 1 || spec.Percent > 100:
		return nil, errCouponSpec{"percent must be between 1 and 100"}
	case spec.Limit < 0:
		return nil, errCouponSpec{"limit must not be negative (0 is no limit)"}
	case spec.Start != nil && spec.End != nil && !spec.End.After(*spec.Start):
		return nil, errCouponSpec{"end must be after start"}
	}
	plans := make([]string, 0, len(spec.Plans))
	seen := map[string]bool{}
	for _, s := range spec.Plans {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		plans = append(plans, s)
	}
	if len(plans) == 0 {
		return nil, errCouponSpec{"plans must name at least one paid plan, such as [\"dev\"]"}
	}
	paid, err := paidPlans(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range plans {
		if !paid[s] {
			return nil, errCouponSpec{fmt.Sprintf("plan %q is not a paid plan on sale", s)}
		}
	}

	mu := couponLock(code)
	mu.Lock()
	defer mu.Unlock()

	db := platform(ctx)
	cpn, _, err := loadCoupon(db, code)
	if err != nil {
		return nil, err
	}
	if cpn != nil && cpn.Filter != coupon.PlanFilter {
		return nil, errCouponSpec{fmt.Sprintf("code %s is held by a coupon that is not a plan coupon", code)}
	}
	var before Map
	event := "coupon.created"
	if cpn == nil {
		cpn = coupon.New(db)
		cpn.Code_ = code
	} else {
		before = viewOf(cpn).audit("")
		event = "coupon.updated"
	}
	cpn.Name = code
	cpn.Type = coupon.Percent
	cpn.Filter = coupon.PlanFilter
	cpn.Once = true
	cpn.Amount = spec.Percent
	cpn.Limit = spec.Limit
	cpn.Plans = plans
	cpn.ProductId = ""
	cpn.Enabled = spec.Enabled
	cpn.StartDate, cpn.EndDate = time.Time{}, time.Time{}
	if spec.Start != nil {
		cpn.StartDate = spec.Start.UTC()
	}
	if spec.End != nil {
		cpn.EndDate = spec.End.UTC()
	}
	if before == nil {
		err = cpn.Create()
	} else {
		err = cpn.Update()
	}
	if err != nil {
		return nil, err
	}

	v := viewOf(cpn)
	if _, err := engine.EmitBillingEvent(db, event, "coupon", code, "", v.audit(by), before); err != nil {
		return nil, fmt.Errorf("coupon %s was stored and its audit record was not: %w", code, err)
	}
	log.Info("plan coupon %s %s by %s: %d%% off, limit %d, plans %v, enabled %v", code, event, by, v.Percent, v.Limit, v.Plans, v.Enabled)
	return &v, nil
}

// paidPlans is every plan slug sold for money: the plan authority's listed rows
// with a price. A contact-sales plan is priced by a person, never by a code, and
// a free plan has nothing to discount. An authority holding none is an error,
// not an empty answer — a seed that did not run must not read as "no such plan".
func paidPlans(ctx context.Context) (map[string]bool, error) {
	rows := make([]*plan.Plan, 0)
	if _, err := plan.Query(plan.AuthorityDB(ctx)).GetAll(&rows); err != nil {
		return nil, fmt.Errorf("the plan catalog could not be read: %w", err)
	}
	paid := map[string]bool{}
	for _, p := range rows {
		if p.Listed() && p.Price > 0 && !p.ContactSales {
			paid[p.Slug] = true
		}
	}
	if len(paid) == 0 {
		return nil, errors.New("the plan catalog holds no paid plan on sale")
	}
	return paid, nil
}

// GetCoupon reads one plan coupon (SuperAdmin only).
//
//	GET /v1/platform/coupons/:code
func GetCoupon(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	v, err := ReadCoupon(c.Context(), c.Param("code"))
	switch {
	case errors.Is(err, ErrCouponNotFound):
		return http.Fail(c, 404, "coupon not found", nil)
	case err != nil:
		return http.Fail(c, 500, "failed to load coupon", err)
	}
	return c.JSON(200, v)
}

// PutCoupon creates or replaces one plan coupon (SuperAdmin only), audited.
//
//	PUT /v1/platform/coupons/:code
//	{"percent":50,"start":"…Z","end":"…Z","limit":5,"plans":["dev"],"enabled":true}
func PutCoupon(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	var spec CouponSpec
	if err := c.Bind(&spec); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	v, err := SetCoupon(c.Context(), c.Param("code"), spec, middleware.Actor(c))
	switch {
	case IsCouponSpecRefused(err):
		return http.Fail(c, 400, err.Error(), nil)
	case err != nil:
		return http.Fail(c, 500, "failed to save coupon", err)
	}
	return c.JSON(200, v)
}
