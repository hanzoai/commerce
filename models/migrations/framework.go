package migrations

type MigrationStatus struct {
	Cursor string
	Done   bool
}

// type delayFn func(c appengine.Context)
// type delayIterFn func(c appengine.Context, iter *datastore.Iterator)
// type migrationFn func(iter *datastore.Iterator)

// func newMigration(c appengine.Context, name, table string) delayIterFn {
// 	log.Info("Migrating "+table, c)

// 	var t *datastore.Iterator

// 	// Try to get cursor if it exists
// 	mk = datastore.NewKey(c, "migrations", name, 0, nil)
// 	if err = datastore.Get(c, mk, &m); err != nil {
// 		log.Warn("No Preexisting Cursor found", c)
// 		t = datastore.NewQuery(table).Run(c)
// 	} else if m.Done {
// 		//Migration Complete
// 		log.Info("Migration was Completed", c)
// 		return
// 	} else if cur, err = datastore.DecodeCursor(m.Cursor); err != nil {
// 		log.Info("Preexisting Cursor is corrupt", c)
// 		t = datastore.NewQuery(table).Run(c)
// 	} else {
// 		log.Info("Resuming from Preexisting Cursor", c)
// 		t = datastore.NewQuery(table).Start(cur).Run(c)
// 	}

// 	return func(c appengine.Context, iter *datastore.Iterator) {
// 	}
// }
