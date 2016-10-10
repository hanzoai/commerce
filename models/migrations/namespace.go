package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/bundle"
	"crowdstart.com/models/collection"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/plan"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var newNamespace = ""

func setupNamespaceRename(c *gin.Context) []interface{} {
	panic("Unable to pass configuration info back to migration funcs yet")

	// TODO: we SHOULD be able to do this
	q := c.Request.URL.Query()
	oldns := q.Get("old-namespace")
	newns := q.Get("new-namespace")

	// Should not need global
	newNamespace = newns

	db := datastore.New(c)

	// Try to find organization
	org := new(organization.Organization)
	ok, err := org.Query().Filter("Name=", oldns).Get()
	if !ok || err != nil {
		panic("Unable to find organization")
	}

	// Update namespace name
	org.Name = newns

	if err = org.Put(); err != nil {
		panic(err)
	}

	// Save new namespace
	ns := namespace.New(db)
	ns.Name = newns
	ns.IntId = org.Key().IntID()
	err = ns.Put()
	if err != nil {
		log.Warn("Failed to put new namespace: %v", err)
	}

	// Set namespace to ensure we iterate over old entities
	c.Set("namespace", oldns)
	return NoArgs
}

// Setup migration
var _ = New("namespace-rename", setupNamespaceRename,
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
