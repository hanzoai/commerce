package migrations

import (
	"appengine"
	"appengine/delay"
)

var migrations = make(map[string][]*delay.Function)

// Add new migration
func addMigration(name string, fns ...*delay.Function) {
	// Create slice for migration set
	if _, ok := migrations[name]; !ok {
		migrations[name] = make([]*delay.Function, 0)
	}

	// Append migration
	migrations[name] = append(migrations[name], fns...)
}

// Run migrations
var Run = delay.Func("run-migration", func(c appengine.Context, name string) {
	fns := migrations[name]
	for _, fn := range fns {
		fn.Call(c)
	}
})

// Define all migrations
func init() {
	// Add email to orders
	addMigration("add-email-to-orders", addEmailToOrders)
}
