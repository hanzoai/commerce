package migration

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/bundle"
	"crowdstart.io/models/collection"
	"crowdstart.io/models/coupon"
	"crowdstart.io/models/mailinglist"
	"crowdstart.io/models/order"
	"crowdstart.io/models/organization"
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

var oldNamespace = "4060001"
var newNamespace = "cyclic"

func setupNamespaceMigration(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	_, err := org.Query().Filter("Name=", "cycliq").First()
	org.Name = oldNamespace
	err = org.Put()
	if err != nil {
		panic(err)
	}
	c.Set("namespace", oldNamespace)
}

var _ = New("namespace", setupNamespaceMigration,
	func(db *ds.Datastore, bundle *bundle.Bundle) {
		bundle.SetNamespace(newNamespace)
		bundle.Put()
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
