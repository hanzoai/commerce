package migrations

import (
	"google.golang.org/appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/log"
)

var _ = New("wipe-search-documents",
	func(c *gin.Context) []interface{} {
		db := datastore.New(c)
		db.SetNamespace("kanoa")
		ctx := db.Context

		index, err := search.Open("user")
		if err != nil {
			log.Error("Failed to open search index for model", ctx)
			return NoArgs
		}

		iter := index.List(ctx, &search.ListOptions{IDsOnly: true})

		for {
			id, err := iter.Next(nil)
			if err != nil {
				break
			}

			index.Delete(ctx, id)
		}

		index, err = search.Open("order")
		if err != nil {
			log.Error("Failed to open search index for model", ctx)
			return NoArgs
		}

		iter = index.List(ctx, &search.ListOptions{IDsOnly: true})

		for {
			id, err := iter.Next(nil)
			if err != nil {
				break
			}

			index.Delete(ctx, id)
		}
		return NoArgs
	},
)
