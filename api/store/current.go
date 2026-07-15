package store

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/store"
)

// getCurrent returns the first (default) store for the authenticated org.
// The admin dashboard calls GET /store/current to resolve the active store context.
func getCurrent(c *zip.Ctx) error {
	ctx := c.Context()
	db := datastore.New(ctx)

	var s store.Store
	s.Init(db)

	q := s.Query().All().Limit(1)

	var stores []store.Store
	if _, err := q.GetAll(&stores); err != nil || len(stores) == 0 {
		// Return a minimal default so the dashboard can render
		return c.JSON(http.StatusOK, map[string]any{
			"store": map[string]any{
				"id":               "default",
				"name":             "Default Store",
				"default_currency": "usd",
				"currencies":       []string{"usd"},
			},
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"store": stores[0]})
}
