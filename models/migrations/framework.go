package migrations

import (
	"runtime/debug"

	"appengine"
	"appengine/datastore"

	"crowdstart.io/util/log"
)

type MigrationStatus struct {
	Cursor string
	Done   bool
}

type migrationFn func(appengine.Context)
type transactionFn func(appengine.Context, *datastore.Key, interface{}) error

func newMigration(name, table string, object interface{}, fn transactionFn) migrationFn {
	return func(c appengine.Context) {
		log.Info("Migrating "+table, c)

		var t *datastore.Iterator
		var m MigrationStatus
		var cur datastore.Cursor
		var k, mk *datastore.Key
		var err error

		// Try to get cursor if it exists
		mk = datastore.NewKey(c, "migration", name, 0, nil)
		if err = datastore.Get(c, mk, &m); err != nil {
			log.Warn("No Preexisting Cursor found", c)
			t = datastore.NewQuery(table).Run(c)
		} else if m.Done {
			//Migration Complete
			log.Info("Migration was Completed", c)
			return
		} else if cur, err = datastore.DecodeCursor(m.Cursor); err != nil {
			log.Info("Preexisting Cursor is corrupt", c)
			t = datastore.NewQuery(table).Run(c)
		} else {
			log.Info("Resuming from Preexisting Cursor", c)
			t = datastore.NewQuery(table).Start(cur).Run(c)
		}

		for {
			// Iterate Cursor
			k, err = t.Next(object)

			if err != nil {
				// Done
				if err == datastore.Done {
					break
				}

				// Ignore field mismatch, otherwise skip record
				if err != nil {
					log.Error("Error fetching user: %v\n%v", k, err, c)
					continue
				}
			}

			// Save Migration point for resume
			mk = datastore.NewKey(c, "migration", name, 0, nil)

			if cur, err = t.Cursor(); err != nil {
				log.Warn("Could not get Cursor because %v", cur, c)
			} else {
				// It doesn't matter if cursor suceeds or not I guess
				datastore.Put(c, mk, &MigrationStatus{Cursor: cur.String(), Done: false})
			}

			log.Info("Migrating Key %v", k, c)
			datastore.RunInTransaction(c, func(tc appengine.Context) error {
				return fn(c, k, object)
			}, &datastore.TransactionOptions{XG: true})

			debug.FreeOSMemory()
		}

		log.Info("Migration Completed", c)
		datastore.Put(c, mk, &MigrationStatus{Cursor: cur.String(), Done: true})
	}
}
