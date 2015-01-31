package migrations

import (
	"appengine"
	"appengine/delay"
)

type simpleFn func(appengine.Context)

var migrations = make(map[string][]*delay.Function)
var migrationFns = make(map[string][]*simpleFn)

// Add new migration
func addMigration(name string, fns ...interface{}) {
	// Create slice for migration set
	if _, ok := migrations[name]; !ok {
		migrations[name] = make([]*delay.Function, 0)
		migrationFns[name] = make([]*simpleFn, 0)
	}

	// Append migration
	for _, fn := range fns {
		switch v := fn.(type) {
		case *delay.Function:
			migrations[name] = append(migrations[name], v)
		case *simpleFn:
			migrationFns[name] = append(migrationFns[name], v)
		}
	}
}

// Run migrations
var Run = delay.Func("run-migration", func(c appengine.Context, name string) {
	dfns := migrations[name]
	for _, dfn := range dfns {
		dfn.Call(c)
	}

	fns := migrationFns[name]
	for _, fn := range fns {
		(*fn)(c)
	}
})

// Define all migrations
func init() {
	// Add email to orders
	addMigration("add-email-to-orders", addEmailToOrders)

	// Replace email with user id
	addMigration("replace-email-with-userid-for-user", replaceEmailWithUserIdForUser)

	// The next 3 depend on replace-email-with-userid-for-user
	addMigration("replace-email-with-userid-for-contribution", replaceEmailWithUserIdForContribution)
	addMigration("replace-email-with-userid-for-invite-token", replaceEmailWithUserIdForInviteToken)
	addMigration("replace-email-with-userid-for-order", replaceEmailWithUserIdForOrder)

	// Create a Entity set of all broken orders
	addMigration("list-broken-orders", listBrokenOrders)

	// Misc clean up
	addMigration("fix-email", fixEmail)
}
