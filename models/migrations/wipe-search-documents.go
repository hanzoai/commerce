package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
)

var _ = New("wipe-search-documents",
	func(c *gin.Context) []interface{} {
		db := datastore.New(c)
		db.SetNamespace("damon")
		ctx := db.Context

		// Search functionality removed - appengine/search is deprecated
		log.Info("wipe-search-documents: search functionality removed (no-op)", ctx)

		return NoArgs
	},
)
