package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/bundle"
	"hanzo.io/models/collection"
	"hanzo.io/models/coupon"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/plan"
	"hanzo.io/models/product"
	"hanzo.io/models/store"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/token"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"

	ds "hanzo.io/datastore"
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
	func(db *ds.Datastore, plan *plan.Plan) {
		plan.Delete()
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
	func(db *ds.Datastore, mailinglist *mailinglist.MailingList) {
		mailinglist.Delete()
	},
	func(db *ds.Datastore, subscriber *subscriber.Subscriber) {
		subscriber.Delete()
	},
)
