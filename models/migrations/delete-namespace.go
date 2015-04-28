package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models/bundle"
	"crowdstart.io/models/collection"
	"crowdstart.io/models/coupon"
	"crowdstart.io/models/mailinglist"
	"crowdstart.io/models/order"
	"crowdstart.io/models/payment"
	"crowdstart.io/models/plan"
	"crowdstart.io/models/product"
	"crowdstart.io/models/store"
	"crowdstart.io/models/subscriber"
	"crowdstart.io/models/token"
	"crowdstart.io/models/user"
	"crowdstart.io/models/variant"

	ds "crowdstart.io/datastore"
)

func setupNamespaceDelete(c *gin.Context) {
	q := c.Request.URL.Query()
	ns := q.Get("namespace")
	if ns == "" {
		panic("Namespace not specified")
	}
	c.Set("namespace", ns)
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
