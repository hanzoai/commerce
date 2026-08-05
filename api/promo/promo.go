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
package promo

import (
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
// handler re-enforces RequirePlatformAdmin — the real boundary. cloud's
// /v1/admin/promos (core.Guard) proxies here with the service token.
func Route(r zip.Router, args ...zip.Handler) {
	api := r.Group("platform")
	api.Use(middleware.TokenRequired(permission.Admin))
	api.Get("/promo", GetPromo)
	api.Put("/promo", PutPromo)
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

// platformDB opens the reserved platform namespace, regardless of the caller's own
// org — the singleton promo is platform-global, read by the public plans view and
// written only by a SuperAdmin.
func platformDB(c *zip.Ctx) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(c.Context(), platformNS))
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
	db := platformDB(c)
	p, err := load(db)
	if err != nil {
		return http.Fail(c, 500, "failed to load promo", err)
	}
	if p == nil {
		return c.JSON(200, Promo{Plans: []string{}})
	}
	return c.JSON(200, toPromo(p))
}

// PutPromo upserts the singleton platform plan promo (SuperAdmin only). percentOff
// is clamped to (0,100]; the window/plans/active are stored on the one Promotion.
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
	if req.PercentOff < 0 || req.PercentOff > 100 {
		return http.Fail(c, 400, "percentOff must be between 0 and 100", nil)
	}
	if req.Start != nil && req.End != nil && req.End.Before(*req.Start) {
		return http.Fail(c, 400, "end must not precede start", nil)
	}

	db := platformDB(c)
	p, err := load(db)
	if err != nil {
		return http.Fail(c, 500, "failed to load promo", err)
	}
	if p == nil {
		p = promotionModel.New(db)
		p.Code = promoCode
	}
	p.Type = promoType
	p.IsAutomatic = true
	p.StartsAt = req.Start
	p.EndsAt = req.End
	if req.Active {
		p.Status = "active"
	} else {
		p.Status = "draft"
	}
	plans := make([]interface{}, 0, len(req.Plans))
	for _, s := range req.Plans {
		if s = strings.TrimSpace(s); s != "" {
			plans = append(plans, s)
		}
	}
	p.Metadata = Map{"percentOff": req.PercentOff, "plans": plans}

	var serr error
	if p.Id() == "" {
		serr = p.Create()
	} else {
		serr = p.Update()
	}
	if serr != nil {
		log.Error("failed to save promo: %v", serr, c)
		return http.Fail(c, 500, "failed to save promo", serr)
	}
	return c.JSON(200, toPromo(p))
}

// Active reports the platform plan promo IF it is active AND within its UTC window
// right now, else nil. This is the ONE gate both the plans view and any pricing
// path share, so a promo that is drafted or expired never discounts. Reads the
// reserved platform namespace so the public (org-less) plans endpoint resolves it.
func Active(c *zip.Ctx) *Promo {
	db := datastore.New(nscontext.WithNamespace(c.Context(), platformNS))
	p, err := load(db)
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
