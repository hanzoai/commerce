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

// Get db namespaced for our fixtures org
func getDb(c *gin.Context) *datastore.Datastore {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)
	org.MustPut()

	log.Debug("Using %s namespace", org.Id())

	// Use org's namespace
	ctx := org.Namespace(c)
	db = datastore.New(ctx)
	return db
}

func init() {
	fixture("fixtures2-organization", Organization)
	fixture("fixtures2-product", Product)
	fixture("fixtures2-token", Token)
	fixture("fixtures2-user", User)
	fixture("fixtures2-variant", Variant)

	// Setup default fixtures
	task.Func("fixtures2-all", func(c *gin.Context) {
		User(c)
		Organization(c)
		Product(c)
		Variant(c)
		Token(c)
	})
}
