package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
)

var _ = New("wipe-search-documents",
	func(c *zip.Ctx) []interface{} {
		db := datastore.New(c.Context())
		db.SetNamespace("damon")
		ctx := db.Context

		// Search functionality removed
		log.Info("wipe-search-documents: search functionality removed (no-op)", ctx)

		return NoArgs
	},
)
