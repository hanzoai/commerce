package migrations

import (
	"github.com/gin-gonic/gin"

	// "google.golang.org/appengine/search"

	// "github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/cart"
	"github.com/hanzoai/commerce/models/order"
	// "github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/demo/tokentransaction"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("rebuild-search-documents",
	func(c *gin.Context) []interface{} {
		db := ds.New(c)

		c.Set("namespace", "damon")
		db.SetNamespace("damon")
		// ctx := db.Context

		// index, err := search.Open(mixin.DefaultIndex)
		// if err != nil {
		// 	log.Error("Failed to open search index for model", ctx)
		// 	return NoArgs
		// }

		// opts := search.SearchOptions{}
		// opts.IDsOnly = true
		// opts.Refinements = []search.Facet{
		// 	search.Facet{
		// 		Name:  "Kind",
		// 		Value: search.Atom("product"),
		// 	},
		// }

		// iter := index.Search(ctx, "", &opts)

		// for {
		// 	id, err := iter.Next(nil)
		// 	if err != nil {
		// 		break
		// 	}

		// 	index.Delete(ctx, id)
		// }

		return NoArgs
	},
	func(db *ds.Datastore, c *cart.Cart) {
		c.PutDocument()
	},
	func(db *ds.Datastore, u *user.User) {
		u.PutDocument()
	},
	func(db *ds.Datastore, t *tokentransaction.Transaction) {
		t.PutDocument()
	},
	func(db *ds.Datastore, o *order.Order) {
		o.PutDocument()
	},
	func(db *ds.Datastore, p *product.Product) {
		p.PutDocument()
	},
)
