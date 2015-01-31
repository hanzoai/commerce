package migrations

import (
	"reflect"

	"appengine"
	"appengine/delay"

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
	addMigration("fix-order-ids", fixOrderIds)
}
