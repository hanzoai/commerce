package migration

import (
	"crowdstart.io/models/bundle"
	"crowdstart.io/models/campaign"
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
	"github.com/gin-gonic/gin"

	ds "crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

var newNamespace = "cyclic"

func setupNamespaceMigration(c *gin.Context) {
	c.Set("namespace", "4060001")
}

var _ = New("namespace", setupNamespaceMigration,
	func(db *ds.Datastore, bundle *bundle.Bundle) {
		bundle.SetNamespace(newNamespace)
		bundle.Put()
	},
	func(db *ds.Datastore, campaign *campaign.Campaign) {
		campaign.SetNamespace(newNamespace)
		campaign.Put()
	},
	func(db *ds.Datastore, collection *collection.Collection) {
		collection.SetNamespace(newNamespace)
		collection.Put()
	},
	func(db *ds.Datastore, coupon *coupon.Coupon) {
		coupon.SetNamespace(newNamespace)
		coupon.Put()
	},
	func(db *ds.Datastore, order *order.Order) {
		order.SetNamespace(newNamespace)
		order.Put()
	},
	func(db *ds.Datastore, payment *payment.Payment) {
		payment.SetNamespace(newNamespace)
		payment.Put()
	},
	func(db *ds.Datastore, plan *plan.Plan) {
		plan.SetNamespace(newNamespace)
		plan.Put()
	},
	func(db *ds.Datastore, product *product.Product) {
		product.SetNamespace(newNamespace)
		product.Put()
	},
	func(db *ds.Datastore, store *store.Store) {
		store.SetNamespace(newNamespace)
		store.Put()
	},
	func(db *ds.Datastore, token *token.Token) {
		token.SetNamespace(newNamespace)
		token.Put()
	},
	func(db *ds.Datastore, variant *variant.Variant) {
		variant.SetNamespace(newNamespace)
		variant.Put()
	},
	func(db *ds.Datastore, user *user.User) {
		log.Warn("%v", user)
		user.SetNamespace(newNamespace)
		user.Put()
	},
	func(db *ds.Datastore, mailinglist *mailinglist.MailingList) {
		mailinglist.SetNamespace(newNamespace)
		mailinglist.Put()
	},
	func(db *ds.Datastore, subscriber *subscriber.Subscriber) {
		subscriber.SetNamespace(newNamespace)
		subscriber.Put()
	},
)
