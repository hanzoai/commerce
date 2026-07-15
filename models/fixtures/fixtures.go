package fixtures

import (
	"reflect"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/task"
)

type Fixture struct {
	fnv    reflect.Value
	entity mixin.Entity
}

func New(name string, fn interface{}) func(c *zip.Ctx) mixin.Entity {
	fix := new(Fixture)

	// Prefix all fixture tasks
	name = "fixtures-" + name

	// Save reference to function
	fix.fnv = reflect.ValueOf(fn)

	// Register task
	task.Func(name, func(c2 *zip.Ctx) error {
		log.Debug("Running %s", name)
		fix.fnv.Call([]reflect.Value{reflect.ValueOf(c2)})
		return nil
	})

	// Return wrapper that memoizes result for safe chaining
	return func(c3 *zip.Ctx) mixin.Entity {
		if fix.entity == nil {
			res := fix.fnv.Call([]reflect.Value{reflect.ValueOf(c3)})
			fix.entity = res[0].Interface().(mixin.Entity)
		}

		return fix.entity
	}
}

// Get db namespaced for our fixtures org
func getNamespaceDb(c *zip.Ctx) *datastore.Datastore {
	org := Organization(c).(*organization.Organization)
	ctx := org.Namespaced(org.Datastore().Context)
	db := datastore.New(ctx)
	return db
}

func init() {
	// Setup default fixtures
	task.Func("fixtures-all", func(c *zip.Ctx) error {
		User(c)
		Organization(c)
		Product(c)
		Plan(c)
		Variant(c)
		Collection(c)
		Token(c)
		Coupon(c)
		Store(c)
		return nil
	})
}
