package fixtures

import (
	"context"
	"reflect"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/mixin"
	"hanzo.io/models/organization"
	"hanzo.io/util/task"
)

type Fixture struct {
	fnv    reflect.Value
	entity mixin.Entity
}

func New(name string, fn interface{}) func(c context.Context) mixin.Entity {
	fix := new(Fixture)

	// Prefix all fixture tasks
	name = "fixtures-" + name

	// Save reference to function
	fix.fnv = reflect.ValueOf(fn)

	// Register task
	task.Func(name, func(c2 *gin.Context) {
		log.Debug("Running %s", name)
		fix.fnv.Call([]reflect.Value{reflect.ValueOf(c2)})
	})

	// Return wrapper that memoizes result for safe chaining
	return func(c3 context.Context) mixin.Entity {
		if fix.entity == nil {
			res := fix.fnv.Call([]reflect.Value{reflect.ValueOf(c3)})
			fix.entity = res[0].Interface().(mixin.Entity)
		}

		return fix.entity
	}
}

// Get db namespaced for our fixtures org
func getNamespaceDb(c context.Context) *datastore.Datastore {
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
		Plan(c)
		Variant(c)
		Collection(c)
		Token(c)
		Coupon(c)
		Campaign(c)
		Store(c)
	})
}
