package migrations

import (
	"github.com/gin-gonic/gin"

	"google.golang.org/appengine/search"

	"hanzo.io/datastore"
	"hanzo.io/log"
	// "hanzo.io/models/order"
	"hanzo.io/models/product"
	// "hanzo.io/models/user"
)

var _ = New("rebuild-search-documents",
	func(c *gin.Context) []interface{} {
		db := datastore.New(c)

		c.Set("namespace", "halcyon")
		db.SetNamespace("halcyon")
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

		return NoArgs
	},
	// func(db *ds.Datastore, u *user.User) {
	// 	u.PutDocument()
	// },
	// func(db *ds.Datastore, o *order.Order) {
	// 	o.PutDocument()
	// },
	func(db datastore.Datastore, p *product.Product) {
		p.PutDocument()
	},
)
