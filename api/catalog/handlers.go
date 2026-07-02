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
//     auth.IAMClaims.GlobalAdmin() — a Hanzo platform admin, never an org-level
//     admin. Wired on the /v1 bundle under /catalog/entries.
//
// The catalog is platform-global: one store in the "system" namespace,
// partitioned by the Brand field, projected per requesting brand.
package catalog

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/catalogentry"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/router"
)

// CatalogNamespace is the platform-global namespace the catalog lives in — not a
// per-tenant org. Brand partitioning is by the entry's Brand field.
const CatalogNamespace = "system"

// catalogDB returns a datastore scoped (via CONTEXT, the only namespacing the
// SQL layer honors) to the platform catalog namespace.
func catalogDB(c *gin.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(middleware.GetContext(c), CatalogNamespace))
}

// requireGlobalAdmin fails closed unless the caller is a Hanzo PLATFORM admin.
// The platform catalog is cross-tenant data, so an org-level admin must NOT edit
// it (Red: org-admin → platform escalation). Returns false + writes 403 on deny.
func requireGlobalAdmin(c *gin.Context) bool {
	claims := iammiddleware.GetIAMClaims(c)
	if claims == nil || !claims.GlobalAdmin() {
		http.Fail(c, 403, "platform admin required to edit the catalog", errors.New("not a global admin"))
		return false
	}
	return true
}

// PublicRoute wires the public, unauthenticated catalog projection. Mount on the
// commerce public group so it serves GET /v1/commerce/catalog.
func PublicRoute(r router.Router) {
	r.GET("/catalog", Public)
}

// AdminRoute wires the platform-admin catalog CRUD + seed on the /v1 bundle.
func AdminRoute(r router.Router, args ...gin.HandlerFunc) {
	g := r.Group("/catalog")
	// args carry the bundle's tokenRequired/adminRequired; each handler ALSO
	// enforces GlobalAdmin() explicitly (defense in depth — a token gate is not
	// a platform-admin gate).
	g.GET("/entries", append(args, ListEntries)...)
	g.POST("/entries", append(args, CreateEntry)...)
	g.PUT("/entries/:slug", append(args, UpdateEntry)...)
	g.DELETE("/entries/:slug", append(args, DeleteEntry)...)
	g.POST("/seed", append(args, SeedCatalog)...)
}

// Public returns the brand-scoped catalog projection. Public + cacheable.
// Brand from ?brand (default hanzo).
func Public(c *gin.Context) {
	brand := c.Query("brand")
	if brand == "" {
		brand = "hanzo"
	}
	cat, err := catalogentry.Project(catalogDB(c), brand)
	if err != nil {
		http.Fail(c, 500, "failed to project catalog", err)
		return
	}
	http.Render(c, 200, cat)
}

// ListEntries returns the raw catalog entries (admin view — includes
// unpublished). Optional ?brand filter.
func ListEntries(c *gin.Context) {
	if !requireGlobalAdmin(c) {
		return
	}
	db := catalogDB(c)
	q := catalogentry.Query(db)
	if brand := c.Query("brand"); brand != "" {
		q = q.Filter("Brand=", brand)
	}
	entries := make([]*catalogentry.CatalogEntry, 0, 128)
	if _, err := q.GetAll(&entries); err != nil {
		http.Fail(c, 500, "failed to list catalog entries", err)
		return
	}
	http.Render(c, 200, entries)
}

// CreateEntry adds a catalog entry (platform admin).
func CreateEntry(c *gin.Context) {
	if !requireGlobalAdmin(c) {
		return
	}
	db := catalogDB(c)
	e := catalogentry.New(db)
	if err := json.Decode(c.Request.Body, e); err != nil {
		http.Fail(c, 400, "failed to decode request body", err)
		return
	}
	if e.Slug == "" {
		http.Fail(c, 400, "slug is required", errors.New("missing slug"))
		return
	}
	if e.Brand == "" {
		e.Brand = "hanzo"
	}
	// Reject a duplicate slug+brand (the projection assumes uniqueness).
	dup := catalogentry.New(db)
	if ok, _ := dup.Query().Filter("Slug=", e.Slug).Filter("Brand=", e.Brand).Get(); ok {
		http.Fail(c, 409, "a catalog entry with this slug already exists for this brand", errors.New("duplicate slug"))
		return
	}
	if err := e.Create(); err != nil {
		http.Fail(c, 500, "failed to create catalog entry", err)
		return
	}
	http.Render(c, 201, e)
}

// UpdateEntry edits a catalog entry by slug (platform admin). The slug + brand
// identity is preserved; other fields are replaced from the body.
func UpdateEntry(c *gin.Context) {
	if !requireGlobalAdmin(c) {
		return
	}
	db := catalogDB(c)
	slug := c.Params.ByName("slug")
	brand := c.Query("brand")
	if brand == "" {
		brand = "hanzo"
	}

	e := catalogentry.New(db)
	ok, err := e.Query().Filter("Slug=", slug).Filter("Brand=", brand).Get()
	if err != nil {
		http.Fail(c, 500, "failed to load catalog entry", err)
		return
	}
	if !ok {
		http.Fail(c, 404, "no catalog entry found with slug: "+slug, errors.New("not found"))
		return
	}

	if err := json.Decode(c.Request.Body, e); err != nil {
		http.Fail(c, 400, "failed to decode request body", err)
		return
	}
	// Identity is immutable — the path slug + brand win over any body value.
	e.Slug = slug
	e.Brand = brand
	if err := e.Update(); err != nil {
		http.Fail(c, 500, "failed to update catalog entry", err)
		return
	}
	http.Render(c, 200, e)
}

// DeleteEntry removes a catalog entry by slug (platform admin).
func DeleteEntry(c *gin.Context) {
	if !requireGlobalAdmin(c) {
		return
	}
	db := catalogDB(c)
	slug := c.Params.ByName("slug")
	brand := c.Query("brand")
	if brand == "" {
		brand = "hanzo"
	}
	e := catalogentry.New(db)
	ok, err := e.Query().Filter("Slug=", slug).Filter("Brand=", brand).Get()
	if err != nil {
		http.Fail(c, 500, "failed to load catalog entry", err)
		return
	}
	if !ok {
		http.Fail(c, 404, "no catalog entry found with slug: "+slug, errors.New("not found"))
		return
	}
	if err := e.Delete(); err != nil {
		http.Fail(c, 500, "failed to delete catalog entry", err)
		return
	}
	c.Data(204, "application/json", make([]byte, 0))
}

// SeedCatalog upserts the embedded Hanzo catalog seed (idempotent,
// non-destructive — never overwrites CMS edits). Platform admin only.
func SeedCatalog(c *gin.Context) {
	if !requireGlobalAdmin(c) {
		return
	}
	created, err := catalogentry.Seed(catalogDB(c))
	if err != nil {
		http.Fail(c, 500, "failed to seed catalog", err)
		return
	}
	http.Render(c, 200, map[string]any{"created": created})
}
