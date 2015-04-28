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

var oldCycliqNamespace = "4060001"
var newCycliqNamespace = "cycliq"

// Update cycliq org to use new namespace, save namespace
func setupCycliqMigration(c *gin.Context) {
	db := datastore.New(c)

	// Try to find organization
	org := new(organization.Organization)
	key, ok, err := db.Query("organization").Filter("Name=", oldCycliqNamespace).First(org)
	if !ok {
		panic("Unable to find organization")
	}

	// Update namespace name
	org.Name = newCycliqNamespace

	key, err = db.PutKind("organization", key, org)
	if err != nil {
		panic(err)
	}

	// Save old namespace
	ns := namespace.New(db)
	ns.Name = oldCycliqNamespace
	ns.IntId = key.IntID()
	err = ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	// Save new namespace
	ns = namespace.New(db)
	ns.Name = newCycliqNamespace
	ns.IntId = key.IntID()
	err = ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	// Set namespace to ensure we iterate over old entities
	c.Set("namespace", oldCycliqNamespace)
}

// Setup migration
var _ = New("cycliq", setupCycliqMigration,
	func(db *ds.Datastore, bundle *bundle.Bundle) {
		bundle.SetNamespace(newCycliqNamespace)
		bundle.Put()
	},
	func(db *ds.Datastore, collection *collection.Collection) {
		collection.SetNamespace(newCycliqNamespace)
		collection.Put()
	},
	func(db *ds.Datastore, coupon *coupon.Coupon) {
		coupon.SetNamespace(newCycliqNamespace)
		coupon.Put()
	},
	func(db *ds.Datastore, order *order.Order) {
		order.SetNamespace(newCycliqNamespace)
		order.Put()
	},
	func(db *ds.Datastore, payment *payment.Payment) {
		payment.SetNamespace(newCycliqNamespace)
		payment.Put()
	},
	func(db *ds.Datastore, plan *plan.Plan) {
		plan.SetNamespace(newCycliqNamespace)
		plan.Put()
	},
	func(db *ds.Datastore, product *product.Product) {
		product.SetNamespace(newCycliqNamespace)
		product.Put()
	},
	func(db *ds.Datastore, store *store.Store) {
		store.SetNamespace(newCycliqNamespace)
		store.Put()
	},
	func(db *ds.Datastore, token *token.Token) {
		token.SetNamespace(newCycliqNamespace)
		token.Put()
	},
	func(db *ds.Datastore, variant *variant.Variant) {
		variant.SetNamespace(newCycliqNamespace)
		variant.Put()
	},
	func(db *ds.Datastore, user *user.User) {
		log.Warn("%v", user)
		user.SetNamespace(newCycliqNamespace)
		user.Put()
	},
	func(db *ds.Datastore, mailinglist *mailinglist.MailingList) {
		mailinglist.SetNamespace(newCycliqNamespace)
		mailinglist.Put()
	},
	func(db *ds.Datastore, subscriber *subscriber.Subscriber) {
		subscriber.SetNamespace(newCycliqNamespace)
		subscriber.Put()
	},
)
