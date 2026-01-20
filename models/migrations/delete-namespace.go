package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/bundle"
	"github.com/hanzoai/commerce/models/collection"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscriber"
	"github.com/hanzoai/commerce/models/token"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/models/variant"

	ds "github.com/hanzoai/commerce/datastore"
)

func setupNamespaceDelete(c *gin.Context) []interface{} {
	q := c.Request.URL.Query()
	ns := q.Get("namespace")
	if ns == "" {
		panic("Namespace not specified")
	}
	c.Set("namespace", ns)
	return NoArgs
}

var _ = New("namespace-delete", setupNamespaceDelete,
	func(db *ds.Datastore, bundle *bundle.Bundle) {
		bundle.Delete()
	},
	func(db *ds.Datastore, collection *collection.Collection) {
		collection.Delete()
	},
	func(db *ds.Datastore, coupon *coupon.Coupon) {
		coupon.Delete()
	},
	func(db *ds.Datastore, order *order.Order) {
		order.Delete()
	},
	func(db *ds.Datastore, payment *payment.Payment) {
		payment.Delete()
	},
	func(db *ds.Datastore, product *product.Product) {
		product.Delete()
	},
	func(db *ds.Datastore, store *store.Store) {
		store.Delete()
	},
	func(db *ds.Datastore, token *token.Token) {
		token.Delete()
	},
	func(db *ds.Datastore, variant *variant.Variant) {
		variant.Delete()
	},
	func(db *ds.Datastore, user *user.User) {
		user.Delete()
	},
	func(db *ds.Datastore, form *form.Form) {
		form.Delete()
	},
	func(db *ds.Datastore, subscriber *subscriber.Subscriber) {
		subscriber.Delete()
	},
)
