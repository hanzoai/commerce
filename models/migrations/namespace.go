package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models/bundle"
	"crowdstart.io/models/collection"
	"crowdstart.io/models/coupon"
	"crowdstart.io/models/mailinglist"
	"crowdstart.io/models/namespace"
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
	"crowdstart.io/util/log"

	ds "crowdstart.io/datastore"
)

var oldNamespace = "cycliq"
var newNamespace = "cycliq2"

func setupNamespaceMigration(c *gin.Context) {
	db := datastore.New(c)

	// Try to find organization
	org := new(organization.Organization)
	key, ok, err := db.Query("organization").Filter("Name=", newNamespace).First(org)
	if !ok {
		panic("Unable to find organization")
	}

	// Save old namespace TODO: only for cycliq, remove
	ns := namespace.New(db)
	ns.Name = oldNamespace
	ns.IntId = key.IntID()
	err = ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	// Save org with new name
	org.Name = newNamespace

	key, err = db.PutKind("organization", key, org)
	if err != nil {
		panic(err)
	}

	// Save new namespace
	ns = namespace.New(db)
	ns.Name = org.Name
	ns.IntId = key.IntID()
	err = ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	// Set namespace to ensure we iterate over old entities
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
