// Package plan is the SuperAdmin HTTP surface for the platform subscription/DNS
// plan authority (models/plan) — the editable runtime source of truth for
// subscription + DNS pricing that admin.hanzo.ai governs.
//
// Two audiences, two auth models (mirrors api/catalog):
//   - PUBLIC read is GET /v1/billing/plans (api/billing) — no auth, cacheable.
//   - PLATFORM-ADMIN write (create/update/delete/seed) mutates the platform-global
//     plan authority (the "system" namespace, NOT a per-tenant org), so it gates
//     on auth.IAMClaims.IsSuperAdmin() — a Hanzo platform admin, never an org
//     admin. Wired on the /v1 bundle under /plans/entries.
//
// The plan authority is platform-global: one store in models/plan.Namespace
// ("system"), keyed by the globally-unique slug. Because the same rows back
// resolveSubscriptionPlan → the internal-ledger charge, the money-adjacent
// guards live here: the slug is IMMUTABLE on update (deprecate-and-create, never
// rename, so a subscription's stored PlanId / a Stripe plan object can never be
// orphaned), and the free($0)-vs-custom(null, ContactSales) distinction is
// preserved as sent (never coerced null→0).
package plan

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// SeedSource yields the authoritative seed rows (the api/billing embed). It is
// injected at the composition root (api.Route) so this package depends only on
// models/plan — the embed source stays owned by api/billing, no import coupling.
type SeedSource func() []*plan.Plan

// planDB returns a datastore scoped (via context, the only namespacing the SQL
// layer honors) to the platform plan-authority namespace.
func planDB(c *zip.Ctx) *datastore.Datastore {
	return plan.AuthorityDB(c.Context())
}

// requireSuperAdmin fails closed unless the caller is a Hanzo PLATFORM admin.
// The plan authority is cross-tenant pricing data, so an org-level admin must NOT
// edit it (Red: org-admin → platform escalation). Writes 403 + returns false on
// deny. Identical predicate to api/catalog — one platform-admin gate definition.
func requireSuperAdmin(c *zip.Ctx) bool {
	claims := iammiddleware.GetIAMClaims(c)
	if claims == nil || !claims.IsSuperAdmin() {
		http.Fail(c, 403, "platform admin required to edit plans", errors.New("not a global admin"))
		return false
	}
	return true
}

// AdminRoute wires the platform-admin plan CRUD + seed on the /v1 bundle. args
// carry the bundle's token/admin middleware; each handler ALSO enforces
// IsSuperAdmin() explicitly (defense in depth — a token gate is not a
// platform-admin gate). seed is the injected embed source.
func AdminRoute(r zip.Router, seed SeedSource, args ...zip.Handler) {
	g := r.Group("/plans")
	g.Get("/entries", append(args, ListEntries)...)
	g.Post("/entries", append(args, CreateEntry)...)
	g.Put("/entries/:slug", append(args, UpdateEntry)...)
	g.Delete("/entries/:slug", append(args, DeleteEntry)...)
	g.Post("/seed", append(args, seedHandler(seed))...)
}

// ListEntries returns the raw plan authority rows (admin view). Platform admin only.
func ListEntries(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := planDB(c)
	entries := make([]*plan.Plan, 0, 64)
	if _, err := plan.Query(db).GetAll(&entries); err != nil {
		return http.Fail(c, 500, "failed to list plans", err)
	}
	return http.Render(c, 200, entries)
}

// CreateEntry adds a plan (platform admin). Slug is required + globally unique.
// The typed Price/PriceAnnual and the ContactSales flag are stored as sent, so
// the free-vs-custom distinction is preserved (never coerced).
func CreateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := planDB(c)
	p := plan.New(db)
	if err := json.DecodeBytes(c.Body(), p); err != nil {
		return http.Fail(c, 400, "failed to decode request body", err)
	}
	if p.Slug == "" {
		return http.Fail(c, 400, "slug is required", errors.New("missing slug"))
	}
	dup := plan.New(db)
	if ok, _ := dup.Query().Filter("Slug=", p.Slug).Get(); ok {
		return http.Fail(c, 409, "a plan with this slug already exists", errors.New("duplicate slug"))
	}
	// A human decided this row, so the published catalog does not get to overwrite
	// it on the next boot. Managed alone could not say that — the seed sets it too.
	p.AdminEdited = true
	p.Managed = true
	if err := p.Create(); err != nil {
		return http.Fail(c, 500, "failed to create plan", err)
	}
	return http.Render(c, 201, p)
}

// UpdateEntry edits a plan by slug (platform admin). The slug is IMMUTABLE — a
// rename is rejected outright (deprecate-and-create instead) so a subscription's
// stored PlanId or a Stripe plan object can never be orphaned by a renamed row.
// Absent body fields keep their stored value (the row is loaded first), so a
// partial edit never silently zeroes Price/ContactSales.
func UpdateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := planDB(c)
	slug := c.Param("slug")

	// Detect a conflicting body slug BEFORE mutating anything.
	var probe struct {
		Slug string `json:"slug"`
	}
	if err := json.DecodeBytes(c.Body(), &probe); err != nil {
		return http.Fail(c, 400, "failed to decode request body", err)
	}
	if probe.Slug != "" && probe.Slug != slug {
		return http.Fail(c, 400, "slug is immutable (deprecate-and-create, never rename)", errors.New("slug change rejected"))
	}

	p := plan.New(db)
	ok, err := p.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to load plan", err)
	}
	if !ok {
		return http.Fail(c, 404, "no plan found with slug: "+slug, errors.New("not found"))
	}

	if err := json.DecodeBytes(c.Body(), p); err != nil {
		return http.Fail(c, 400, "failed to decode request body", err)
	}
	p.Slug = slug    // identity is immutable — the path slug wins over any body value
	// Same: this value is now a human's decision, and the reconciling seed leaves
	// it alone — including the archive sweep, since a plan an admin added on
	// purpose is not "missing from the catalog", it is theirs.
	p.AdminEdited = true
	p.Managed = true
	if err := p.Update(); err != nil {
		return http.Fail(c, 500, "failed to update plan", err)
	}
	return http.Render(c, 200, p)
}

// DeleteEntry removes a plan by slug (platform admin).
func DeleteEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := planDB(c)
	slug := c.Param("slug")
	p := plan.New(db)
	ok, err := p.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to load plan", err)
	}
	if !ok {
		return http.Fail(c, 404, "no plan found with slug: "+slug, errors.New("not found"))
	}
	if err := p.Delete(); err != nil {
		return http.Fail(c, 500, "failed to delete plan", err)
	}
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(204, make([]byte, 0))
}

// seedHandler upserts the embedded plan catalog (idempotent, non-destructive —
// never overwrites an admin edit/delete). Platform admin only.
func seedHandler(seed SeedSource) zip.Handler {
	return func(c *zip.Ctx) error {
		if !requireSuperAdmin(c) {
			return nil
		}
		if seed == nil {
			return http.Fail(c, 500, "no plan seed source wired", errors.New("nil seed source"))
		}
		created, corrected, err := plan.Seed(planDB(c), seed())
		if err != nil {
			return http.Fail(c, 500, "failed to seed plans", err)
		}
		return http.Render(c, 200, map[string]any{"created": created, "corrected": corrected})
	}
}
