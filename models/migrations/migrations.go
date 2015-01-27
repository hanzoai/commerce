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

	// Replace email with user id
	addMigration("replace-email-with-userid-for-user", replaceEmailWithUserIdForUser)

	// The next 3 depend on replace-email-with-userid-for-user
	addMigration("replace-email-with-userid-for-contribution", replaceEmailWithUserIdForContribution)
	addMigration("replace-email-with-userid-for-invite-token", replaceEmailWithUserIdForInviteToken)
	addMigration("replace-email-with-userid-for-order", replaceEmailWithUserIdForOrder)

	// Create a Entity set of all broken orders
	addMigration("list-broken-orders", listBrokenOrders)
	addMigration("fix-email", fixEmail)
}
