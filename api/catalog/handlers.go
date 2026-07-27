// Package catalog is the HTTP surface for the platform product catalog — the
// CMS source-of-truth for a brand's OWN products (Models, Vector, KMS, …) that
// docs.<brand>, the console sidebar, and pricing derive from.
//
// Two audiences, two auth models:
//   - PUBLIC read: GET /catalog returns the brand-scoped projection with no
//     auth (it is public presentation + pricing data). Wired on the commerce
//     public group so it serves the exact path GET /v1/commerce/catalog.
//   - PLATFORM-ADMIN write: create/update/delete/seed mutate the platform-global
//     catalog (the "system" namespace, NOT a per-tenant org), so they gate on
//     auth.IAMClaims.IsSuperAdmin() — a Hanzo platform admin, never an org-level
//     admin. Wired on the /v1 bundle under /catalog/entries.
//
// The catalog is platform-global: one store in the "system" namespace, scoped
// per requesting brand BY CATEGORY at projection time (matching @hanzo/products
// catalogForBrand). Entries are keyed by their globally-unique slug.
package catalog

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/catalogentry"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// catalogDB returns a datastore scoped to the platform catalog namespace.
// catalogentry.SystemDB owns that scoping — there is one way to reach the store.
func catalogDB(c *zip.Ctx) *datastore.Datastore {
	return catalogentry.SystemDB(c.Context())
}

// requireSuperAdmin fails closed unless the caller is a Hanzo PLATFORM admin.
// The platform catalog is cross-tenant data, so an org-level admin must NOT edit
// it (Red: org-admin → platform escalation). Returns false + writes 403 on deny.
func requireSuperAdmin(c *zip.Ctx) bool {
	claims := iammiddleware.GetIAMClaims(c)
	if claims == nil || !claims.IsSuperAdmin() {
		http.Fail(c, 403, "platform admin required to edit the catalog", errors.New("not a global admin"))
		return false
	}
	return true
}

// PublicRoute wires the public, unauthenticated catalog projection. Mount on the
// commerce public group so it serves GET /v1/commerce/catalog.
func PublicRoute(r zip.Router) {
	r.Get("/catalog", Public)
}

// AdminCatalogRoute wires the owner=="admin" margin projection. Mount on an
// IAM-gated commerce group so it serves GET /v1/commerce/admin/catalog; the
// handler ALSO enforces IsSuperAdmin() (defense in depth).
func AdminCatalogRoute(r zip.Router) {
	r.Get("/admin/catalog", AdminCatalog)
}

// AdminRoute wires the platform-admin catalog CRUD + seed on the /v1 bundle.
func AdminRoute(r zip.Router, args ...zip.Handler) {
	g := r.Group("/catalog")
	// args carry the bundle's tokenRequired/adminRequired; each handler ALSO
	// enforces IsSuperAdmin() explicitly (defense in depth — a token gate is not
	// a platform-admin gate).
	g.Get("/entries", append(args, ListEntries)...)
	g.Post("/entries", append(args, CreateEntry)...)
	// The slug is a WILDCARD, not a segment param, because a model's slug IS its
	// callable id and those contain a slash ("anthropic/claude-opus-5"). A :slug
	// param stops at the slash, so every model row would be listed and
	// un-editable — a catalog whose own admin surface cannot address most of its
	// rows. Measured on this router: fiber's `+` param does not bind here, `*`
	// does, and it matches a single-segment slug identically, so the infra-tier
	// edits are byte-for-byte unchanged.
	g.Put("/entries/*", append(args, UpdateEntry)...)
	g.Delete("/entries/*", append(args, DeleteEntry)...)
	g.Post("/seed", append(args, SeedCatalog)...)
	g.Post("/models", append(args, SyncModels)...)
	g.Post("/models/refresh", append(args, RefreshModels)...)
}

// entrySlug reads the addressed slug from the wildcard path.
func entrySlug(c *zip.Ctx) string { return c.Param("*") }

// Public returns the brand-scoped catalog projection. Public + cacheable.
// Brand from ?brand (default hanzo).
func Public(c *zip.Ctx) error {
	brand := c.Query("brand")
	if brand == "" {
		brand = "hanzo"
	}
	cat, err := catalogentry.Project(catalogDB(c), brand)
	if err != nil {
		return http.Fail(c, 500, "failed to project catalog", err)
	}
	return http.Render(c, 200, cat)
}

// AdminCatalog returns the brand-scoped catalog projection WITH the administrative
// economics (costCents + marginPct) the public projection withholds — the margin
// surface admin.hanzo.ai administrates. owner=="admin" only (IsSuperAdmin); an
// org-level admin is refused 403, so upstream cost and margin never leak to a
// tenant. Brand from ?brand (default hanzo).
func AdminCatalog(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	brand := c.Query("brand")
	if brand == "" {
		brand = "hanzo"
	}
	cat, err := catalogentry.ProjectAdmin(catalogDB(c), brand)
	if err != nil {
		return http.Fail(c, 500, "failed to project admin catalog", err)
	}
	return http.Render(c, 200, cat)
}

// ListEntries returns the raw catalog entries (admin view — includes
// unpublished). Optional ?brand filter.
func ListEntries(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := catalogDB(c)
	entries := make([]*catalogentry.CatalogEntry, 0, 128)
	if _, err := catalogentry.Query(db).GetAll(&entries); err != nil {
		return http.Fail(c, 500, "failed to list catalog entries", err)
	}
	return http.Render(c, 200, entries)
}

// CreateEntry adds a catalog entry (platform admin).
func CreateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := catalogDB(c)
	e := catalogentry.New(db)
	if err := json.DecodeBytes(c.Body(), e); err != nil {
		return http.Fail(c, 400, "failed to decode request body", err)
	}
	if e.Slug == "" {
		return http.Fail(c, 400, "slug is required", errors.New("missing slug"))
	}
	// Slug is the globally-unique catalog key — reject a duplicate.
	dup := catalogentry.New(db)
	if ok, _ := dup.Query().Filter("Slug=", e.Slug).Get(); ok {
		return http.Fail(c, 409, "a catalog entry with this slug already exists", errors.New("duplicate slug"))
	}
	if err := e.Create(); err != nil {
		return http.Fail(c, 500, "failed to create catalog entry", err)
	}
	return http.Render(c, 201, e)
}

// UpdateEntry edits a catalog entry by slug (platform admin). The slug identity
// is preserved; other fields are replaced from the body.
func UpdateEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := catalogDB(c)
	slug := entrySlug(c)

	e := catalogentry.New(db)
	ok, err := e.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to load catalog entry", err)
	}
	if !ok {
		return http.Fail(c, 404, "no catalog entry found with slug: "+slug, errors.New("not found"))
	}

	if err := json.DecodeBytes(c.Body(), e); err != nil {
		return http.Fail(c, 400, "failed to decode request body", err)
	}
	e.Slug = slug // identity is immutable — the path slug wins over any body value
	if err := e.Update(); err != nil {
		return http.Fail(c, 500, "failed to update catalog entry", err)
	}
	return http.Render(c, 200, e)
}

// DeleteEntry removes a catalog entry by slug (platform admin).
func DeleteEntry(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	db := catalogDB(c)
	slug := entrySlug(c)
	e := catalogentry.New(db)
	ok, err := e.Query().Filter("Slug=", slug).Get()
	if err != nil {
		return http.Fail(c, 500, "failed to load catalog entry", err)
	}
	if !ok {
		return http.Fail(c, 404, "no catalog entry found with slug: "+slug, errors.New("not found"))
	}
	if err := e.Delete(); err != nil {
		return http.Fail(c, 500, "failed to delete catalog entry", err)
	}
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(204, make([]byte, 0))
}

// SyncModels lands a syncer's view of the model catalog: it refreshes each
// model's upstream COST and machine-observable facts and touches nothing a
// human owns — not the retail price, not the markup, not the entitlement tier
// (catalogentry.UpsertModels enforces that, so the rule holds no matter which
// syncer calls). This is the ONE write seam between the model families /
// upstream and the catalog: they own the structure and publish it, commerce
// holds the numbers, admin.hanzo.ai edits them. Platform admin only.
func SyncModels(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	var rows []catalogentry.ModelRow
	if err := json.DecodeBytes(c.Body(), &rows); err != nil {
		return http.Fail(c, 400, "failed to decode model rows", err)
	}
	stats, err := catalogentry.UpsertModels(catalogDB(c), rows)
	if err != nil {
		return http.Fail(c, 500, "failed to sync models", err)
	}
	return http.Render(c, 200, stats)
}

// SeedCatalog upserts the embedded Hanzo catalog seed (idempotent,
// non-destructive — never overwrites CMS edits). Platform admin only.
func SeedCatalog(c *zip.Ctx) error {
	if !requireSuperAdmin(c) {
		return nil
	}
	created, err := catalogentry.Seed(catalogDB(c))
	if err != nil {
		return http.Fail(c, 500, "failed to seed catalog", err)
	}
	return http.Render(c, 200, map[string]any{"created": created})
}
