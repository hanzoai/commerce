package fixtures

import (
	"reflect"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

// Add a fixture as a registered task
func fixture(name string, fn interface{}) {
	fnv := reflect.ValueOf(fn)
	task.Func(name, func(c *gin.Context) {
		log.Debug("Running %s", name)
		fnv.Call([]reflect.Value{reflect.ValueOf(c)})
	})
}

func getOrg(c *gin.Context) *organization.Organization {
	db := datastore.New(c)
	org := organization.New(db)
	org.Name = "suchtees"
	org.Query().Filter("Name=", org.Name).First()
	return org
}

// Get db namespaced for our fixtures org
func getDb(c *gin.Context) *datastore.Datastore {
	org := getOrg(c)

	log.Debug("Using (%v,%s) namespace", org.Key(), org.Name)

	// Use org's namespace
	ctx := org.Namespace(c)
	db := datastore.New(ctx)
	return db
}

func init() {
	fixture("fixtures-campaign", Campaign)
	fixture("fixtures-coupon", Coupon)
	fixture("fixtures-collection", Collection)
	fixture("fixtures-organization", Organization)
	fixture("fixtures-product", Product)
	fixture("fixtures-token", Token)
	fixture("fixtures-user", User)
	fixture("fixtures-variant", Variant)
	fixture("fixtures-cycliq", Cycliq)

	// Setup default fixtures
	task.Func("fixtures-all", func(c *gin.Context) {
		Organization(c)
		Product(c)
		Variant(c)
		Collection(c)
		Token(c)
		Coupon(c)
		Campaign(c)
	})
}
