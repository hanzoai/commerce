package fixtures

import (
	"reflect"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/organization"
	"hanzo.io/util/log"
	"hanzo.io/util/task"
)

type Fixture struct {
	fnv    reflect.Value
	entity mixin.Entity
}

func New(name string, fn interface{}) func(c *gin.Context) mixin.Entity {
	fix := new(Fixture)

	// Prefix all fixture tasks
	name = "fixtures-" + name

	// Save reference to function
	fix.fnv = reflect.ValueOf(fn)

	// Register task
	task.Func(name, func(c *gin.Context) {
		log.Debug("Running %s", name)
		fix.fnv.Call([]reflect.Value{reflect.ValueOf(c)})
	})

	// Return wrapper that memoizes result for safe chaining
	return func(c *gin.Context) mixin.Entity {
		if fix.entity == nil {
			res := fix.fnv.Call([]reflect.Value{reflect.ValueOf(c)})
			fix.entity = res[0].Interface().(mixin.Entity)
		}

		return fix.entity
	}
}

// Get db namespaced for our fixtures org
func getNamespaceDb(c *gin.Context) *datastore.Datastore {
	org := Organization(c).(*organization.Organization)
	ctx := org.Namespaced(org.Db.Context)
	db := datastore.New(ctx)
	return db
}

func init() {
	// Setup default fixtures
	task.Func("fixtures-all", func(c *gin.Context) {
		User(c)
		Organization(c)
		Product(c)
		Variant(c)
		Collection(c)
		Token(c)
		Coupon(c)
		Campaign(c)
		Store(c)
	})
}
