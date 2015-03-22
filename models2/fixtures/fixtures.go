package fixtures

import (
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/task"
)

// Add a fixture as a registered task
func fixture(name string, fn interface{}) {
	fnv := reflect.ValueOf(fn)
	task.Func(name, func(c *gin.Context) {
		fnv.Call([]reflect.Value{reflect.ValueOf(c)})
	})
}

// Get db namespaced for our fixtures org
func getDb(c *gin.Context) *datastore.Datastore {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

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

	// Register all fixtures under a fixtures-all task name
	for name, tasks := range task.Registry {
		if strings.HasPrefix(name, "fixtures2-") {
			task.Register("fixtures2-all", tasks...)
		}
	}
}
