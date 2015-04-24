package migration

import (
	"crowdstart.io/models2/bundle"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/collection"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/plan"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/store"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/models2/token"
	"crowdstart.io/models2/user"
	"crowdstart.io/models2/variant"
	"github.com/gin-gonic/gin"

	"appengine"

	ds "crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
)

var newNamespace = "cyclic"

func setupNamespaceMigration(c *gin.Context) {
	if ctx, err := appengine.Namespace(middleware.GetAppEngine(c), "2"); err != nil {
		log.Error("Could not namespace in dispatch: %v", err, ctx)
		return
	} else {
		c.Set("appengine", ctx)
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
