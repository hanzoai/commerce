// Copyright © 2026 Hanzo AI. MIT License.

package store

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/store"
)

// getCurrent returns the authenticated org's default store. It resolves the store
// from the org's OWN namespaced datastore (NOT the shared system DB) and lazily
// provisions it on first use. The admin dashboard and the content storefront edge
// both call GET /store/current to resolve the active store id; reading the shared
// system DB always returned the phantom "default", which the storefront edge treats
// as unconfigured — so it skipped publishing every org's product Listing.headerImage.
func getCurrent(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok {
		// No authenticated org in context: return a minimal default so the dashboard
		// still renders, exactly as before. The route now runs behind tokenRequired,
		// so real callers (dashboard IAM, storefront service token) always have an org.
		return c.JSON(http.StatusOK, defaultStorePayload())
	}

	// Resolve the caller org's OWN store — the SAME per-org namespace the write path
	// (listing.go orgNamespacedDB, rest.newEntity) persists into.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	// A selected store is explicit. Resolve it only inside this org's namespace;
	// a foreign id therefore cannot cross the tenant boundary.
	if c.Header("X-Store-Id") != "" {
		s, err := resolveStore(c, db)
		if err != nil || s == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "store not found"})
		}
		return c.JSON(http.StatusOK, map[string]any{"store": s})
	}

	// Return the org's existing store if it already has one (any slug).
	var stores []store.Store
	if _, err := store.New(db).Query().All().Limit(1).GetAll(&stores); err == nil && len(stores) > 0 {
		return c.JSON(http.StatusOK, map[string]any{"store": stores[0]})
	}

	// First authenticated visit for an org with no store yet: lazily provision the
	// canonical default store (idempotent, org-scoped, no payment creds) so the
	// storefront edge resolves a REAL store id instead of the phantom "default".
	s, err := store.EnsureDefault(db)
	if err != nil {
		return c.JSON(http.StatusOK, defaultStorePayload())
	}
	return c.JSON(http.StatusOK, map[string]any{"store": s})
}

// defaultStorePayload is the minimal fallback used only when no org context is
// present or provisioning fails — never the normal path for an authenticated org.
func defaultStorePayload() map[string]any {
	return map[string]any{
		"store": map[string]any{
			"id":               "default",
			"name":             "Default Store",
			"default_currency": "usd",
			"currencies":       []string{"usd"},
		},
	}
}
