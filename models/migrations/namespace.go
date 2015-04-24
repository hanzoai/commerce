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

	"appengine"

	ds "crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
)

var newNamespace = "suchtees"

func setupNamespaceMigration(c *gin.Context) {
	if ctx, err := appengine.Namespace(middleware.GetAppEngine(c), "2"); err != nil {
		log.Error("Could not namespace in dispatch: %v", err, ctx)
		return
	} else {
		c.Set("appengine", ctx)
		c.Set("namespace", "2")
	}
}

var _ = New("namespace", setupNamespaceMigration,
	func(db *ds.Datastore, key ds.Key, bundle *bundle.Bundle) {
		bundle.SetNamespace(newNamespace)
		bundle.Put()
	},
	func(db *ds.Datastore, key ds.Key, campaign *campaign.Campaign) {
		campaign.SetNamespace(newNamespace)
		campaign.Put()
	},
	func(db *ds.Datastore, key ds.Key, collection *collection.Collection) {
		collection.SetNamespace(newNamespace)
		collection.Put()
	},
	func(db *ds.Datastore, key ds.Key, coupon *coupon.Coupon) {
		coupon.SetNamespace(newNamespace)
		coupon.Put()
	},
	func(db *ds.Datastore, key ds.Key, order *order.Order) {
		order.SetNamespace(newNamespace)
		order.Put()
	},
	func(db *ds.Datastore, key ds.Key, payment *payment.Payment) {
		payment.SetNamespace(newNamespace)
		payment.Put()
	},
	func(db *ds.Datastore, key ds.Key, plan *plan.Plan) {
		plan.SetNamespace(newNamespace)
		plan.Put()
	},
	func(db *ds.Datastore, key ds.Key, product *product.Product) {
		product.SetNamespace(newNamespace)
		product.Put()
	},
	func(db *ds.Datastore, key ds.Key, store *store.Store) {
		store.SetNamespace(newNamespace)
		store.Put()
	},
	func(db *ds.Datastore, key ds.Key, token *token.Token) {
		token.SetNamespace(newNamespace)
		token.Put()
	},
	func(db *ds.Datastore, key ds.Key, variant *variant.Variant) {
		variant.SetNamespace(newNamespace)
		variant.Put()
	},
	func(db *ds.Datastore, key ds.Key, user *user.User) {
		log.Warn("%v", user)
		user.SetNamespace(newNamespace)
		user.Put()
	},
	func(db *ds.Datastore, key ds.Key, mailinglist *mailinglist.MailingList) {
		mailinglist.SetNamespace(newNamespace)
		mailinglist.Put()
	},
	func(db *ds.Datastore, key ds.Key, subscriber *subscriber.Subscriber) {
		subscriber.SetNamespace(newNamespace)
		subscriber.Put()
	},
)
