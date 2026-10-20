// Package promo is the platform PLAN promo — the admin-controlled "50% off for a
// limited time" offer, composed on the existing Promotion model (NOT a parallel
// model) so the discount that a customer sees on GET /v1/billing/plans is
// SuperAdmin-configured, never hardcoded in the catalog.
//
// It is a SINGLETON: one platform-wide plan promo, stored as one Promotion in the
// reserved platform ("admin") namespace, so the public plans endpoint can surface
// it without a per-org lookup and the SuperAdmin edits it in one place. percentOff
// + the applicable plan slugs ride the Promotion's Metadata; the window is the
// Promotion's StartsAt/EndsAt; `active` is its Status. This is the canonical
// {percentOff, start, end, plans, active} shape (openapi admin Promo).
//
// Beside it live the plan COUPONS (coupon.go): a code a buyer types for a percent
// off the first period of one plan purchase, stored in the same namespace and
// edited on the same SuperAdmin surface.
package promo

import (
	"context"
	"errors"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	promotionModel "github.com/hanzoai/commerce/models/promotion"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"

	. "github.com/hanzoai/commerce/types"
)

// Route mounts the SuperAdmin plan-promo surface at /v1/platform/promo. Like the
// metrics god-view the route-level token gate is a NO-OP on the IAM path, so each
// handler re-enforces RequirePlatformAdmin — the real boundary. This is the
// standalone binary's face; cloud's /v1/admin/promos asks ReadPromo and SetPromo
// over its internal plane, where the store is.
func Route(r *zip.Group, args ...zip.Handler) {
	api := r.Group("platform")
	api.Use(middleware.TokenRequired(permission.Admin))
	api.Raw(nethttp.MethodGet, "/promo", GetPromo)
	api.Raw(nethttp.MethodPut, "/promo", PutPromo)
	api.Raw(nethttp.MethodGet, "/coupons/:code", GetCoupon)
	api.Raw(nethttp.MethodPut, "/coupons/:code", PutCoupon)
}

const (
	// platformNS is the reserved platform namespace the singleton promo lives in —
	// the "admin" org (owner=="admin" is the SuperAdmin predicate everywhere), so a
	// platform-wide promo is stored where only platform admins write and the public
	// plans view reads.
	platformNS = "admin"

	// promoCode is the well-known code of the ONE platform plan promo, so load/save
	// resolve the same singleton row every time.
	promoCode = "platform-plan-promo"

	// promoType marks the Promotion as the plan-scoped percent-off promo (distinct
	// from cart/order promotions the store Evaluate path handles).
	promoType = "plan_percent_off"
)

// Promo is the canonical wire shape: percentOff (whole percent 1..100), the UTC
// window [start,end], the applicable plan slugs (empty = ALL paid plans), and
// whether it is active. It maps 1:1 to the openapi admin Promo type.
type Promo struct {
	PercentOff int        `json:"percentOff"`
	Start      *time.Time `json:"start,omitempty"`
	End        *time.Time `json:"end,omitempty"`
	Plans      []string   `json:"plans"`
	Active     bool       `json:"active"`
}

// platform opens the reserved platform namespace, regardless of the caller's own
// org: the singleton promo and the plan coupons are platform-global, read by the
// public plans view and the sale, and written only by a SuperAdmin.
func platform(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, platformNS))
}

// load reads the singleton platform plan-promo, or nil when none is configured.
func load(db *datastore.Datastore) (*promotionModel.Promotion, error) {
	p := promotionModel.New(db)
	ok, err := p.Query().Filter("Code=", promoCode).Get()
	if err != nil || !ok {
		return nil, err
	}
	return p, nil
}

// toPromo projects a stored Promotion onto the wire shape.
func toPromo(p *promotionModel.Promotion) Promo {
	pr := Promo{
		Start:  p.StartsAt,
		End:    p.EndsAt,
		Active: strings.EqualFold(p.Status, "active"),
		Plans:  []string{},
	}
	if p.Metadata != nil {
		if v, ok := p.Metadata["percentOff"].(float64); ok {
			pr.PercentOff = int(v)
		}
		if raw, ok := p.Metadata["plans"].([]interface{}); ok {
			for _, s := range raw {
				if str, ok := s.(string); ok && strings.TrimSpace(str) != "" {
					pr.Plans = append(pr.Plans, strings.TrimSpace(str))
				}
			}
		}
	}
	return pr
}

// GetPromo returns the current platform plan promo (SuperAdmin only). When none is
// configured it returns a zeroed, inactive promo so the admin UI always has a shape
// to edit.
//
//	GET /v1/platform/promo
func GetPromo(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	p, err := ReadPromo(c.Context())
	if err != nil {
		return http.Fail(c, 500, "failed to load promo", err)
	}
	return c.JSON(200, p)
}

// PutPromo upserts the singleton platform plan promo (SuperAdmin only).
//
//	PUT /v1/platform/promo
func PutPromo(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	var req Promo
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	p, err := SetPromo(c.Context(), req)
	switch {
	case IsPromoSpecRefused(err):
		return http.Fail(c, 400, err.Error(), nil)
	case err != nil:
		log.Error("failed to save promo: %v", err, c)
		return http.Fail(c, 500, "failed to save promo", err)
	}
	return c.JSON(200, p)
}

// errPromoSpec is a promo the singleton cannot be stored as.
type errPromoSpec struct{ msg string }

func (e errPromoSpec) Error() string { return e.msg }

// IsPromoSpecRefused reports whether SetPromo refused the promo itself — a percent
// outside 0-100 or a window that ends before it starts — rather than failing to
// store it.
func IsPromoSpecRefused(err error) bool {
	var e errPromoSpec
	return errors.As(err, &e)
}

// ReadPromo returns the platform plan promo as stored, whether or not it is on
// sale now (Current answers that), or a zeroed, inactive promo when none is
// configured. It is the value-taking core GetPromo serves and the internal plane
// asks, so a peer that holds no store reads the one row the same way.
func ReadPromo(ctx context.Context) (Promo, error) {
	p, err := load(platform(ctx))
	if err != nil {
		return Promo{}, err
	}
	if p == nil {
		return Promo{Plans: []string{}}, nil
	}
	return toPromo(p), nil
}

// SetPromo replaces the platform plan promo with in, creating the singleton when
// there is none: percentOff 0-100, the window [start, end] (an end before the
// start is refused), the plan slugs it covers (empty is every paid plan) and
// whether it is on. It returns the promo as stored.
func SetPromo(ctx context.Context, in Promo) (Promo, error) {
	if in.PercentOff < 0 || in.PercentOff > 100 {
		return Promo{}, errPromoSpec{"percentOff must be between 0 and 100"}
	}
	if in.Start != nil && in.End != nil && in.End.Before(*in.Start) {
		return Promo{}, errPromoSpec{"end must not precede start"}
	}
	db := platform(ctx)
	p, err := load(db)
	if err != nil {
		return Promo{}, err
	}
	if p == nil {
		p = promotionModel.New(db)
		p.Code = promoCode
	}
	p.Type = promoType
	p.IsAutomatic = true
	p.StartsAt = in.Start
	p.EndsAt = in.End
	if in.Active {
		p.Status = "active"
	} else {
		p.Status = "draft"
	}
	plans := make([]interface{}, 0, len(in.Plans))
	for _, s := range in.Plans {
		if s = strings.TrimSpace(s); s != "" {
			plans = append(plans, s)
		}
	}
	p.Metadata = Map{"percentOff": in.PercentOff, "plans": plans}
	if p.Id() == "" {
		err = p.Create()
	} else {
		err = p.Update()
	}
	if err != nil {
		return Promo{}, err
	}
	return toPromo(p), nil
}

// Active reports the platform plan promo IF it is active AND within its UTC window
// right now, else nil. This is the ONE gate both the plans view and any pricing
// path share, so a promo that is drafted or expired never discounts. Reads the
// reserved platform namespace so the public (org-less) plans endpoint resolves it.
//
// It is the request-shaped face of Current, and holds nothing of its own.
func Active(c *zip.Ctx) *Promo { return Current(c.Context()) }

// Current is the same offer, asked WITHOUT a request.
//
// The plan catalog is read over the internal plane by a peer that holds no
// store, and a catalog priced without the offer is a different catalog — so the
// question has to be askable from a context alone. Nothing here was ever
// request-shaped: the promo lives in the reserved PLATFORM namespace, which is
// the same row for every caller, which is also why the public org-less plans
// endpoint can resolve it.
func Current(ctx context.Context) *Promo {
	p, err := load(platform(ctx))
	if err != nil || p == nil {
		return nil
	}
	pr := toPromo(p)
	if !pr.Active || pr.PercentOff <= 0 {
		return nil
	}
	now := time.Now().UTC()
	if pr.Start != nil && now.Before(*pr.Start) {
		return nil
	}
	if pr.End != nil && now.After(*pr.End) {
		return nil
	}
	return &pr
}

// Name is how the promo reads on a receipt — the ONE place a percent becomes a
// label, so the line a customer sees on an invoice cannot drift from the offer
// the plans page advertised. Empty for a promo that discounts nothing, because a
// zero-value discount must not print a name beside it.
func (p *Promo) Name() string {
	if p == nil || p.PercentOff <= 0 {
		return ""
	}
	return strconv.Itoa(p.PercentOff) + "% off"
}

// AppliesTo reports whether the promo discounts a given plan slug: an empty Plans
// list means ALL plans, otherwise only the listed slugs.
func (p *Promo) AppliesTo(slug string) bool {
	if len(p.Plans) == 0 {
		return true
	}
	for _, s := range p.Plans {
		if s == slug {
			return true
		}
	}
	return false
}
