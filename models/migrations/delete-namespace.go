package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/bundle"
	"crowdstart.com/models/collection"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/plan"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"

	ds "crowdstart.com/datastore"
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
