package migrations

import (
	"reflect"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models/migrations/tasks"
	"crowdstart.io/util/log"
)

var migrations = make(map[string][]interface{})

// Add new migration
func addMigration(name string, fns ...interface{}) {
	// Create slice for migration set
	if _, ok := migrations[name]; !ok {
		migrations[name] = make([]interface{}, 0)
	}

	// Append migration
	for _, fn := range fns {
		migrations[name] = append(migrations[name], fn)
	}
}

// Run migrations
var Run = delay.Func("run-migration", func(c appengine.Context, name string) {
	fns := migrations[name]
	for _, fn := range fns {
		switch v := fn.(type) {
		case *delay.Function:
			v.Call(c)
		case func(appengine.Context):
			v(c)
		default:
			log.Error("Couldn't execute %v", reflect.ValueOf(v).Type(), c)
		}
	}
})

// Define all migrations
func init() {
	// Add email to orders
	addMigration("add-email-to-orders", addEmailToOrders)

	// Add email back to contribution
	addMigration("add-email-to-contribution", func(c appengine.Context) {
		parallel.Run(c, "contribution", 100, tasks.AddEmailToContribution)
	})

	// Add missing orders for each contributors
	addMigration("add-missing-orders", func(c appengine.Context) {
		parallel.Run(c, "contribution", 50, tasks.AddMissingOrders)
	})

	// Add missing orders for each contributors
	addMigration("add-id-to-order", func(c appengine.Context) {
		parallel.Run(c, "order", 50, tasks.AddIdToOrder)
	})

	// Create a Entity set of all broken orders
	addMigration("list-broken-orders", listBrokenOrders)

	// Misc clean up
	addMigration("fix-email", fixEmail)
}
