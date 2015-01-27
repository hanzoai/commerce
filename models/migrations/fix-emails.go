package migrations

import (
	"errors"
	"strings"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var fixEmail = delay.Func(
	"migration-fix-email",
	newMigration(
		"migration-fix-email",
		"user",
		new(User),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch u := object.(type) {
			case *User:
				u.Email = strings.ToLower(strings.TrimSpace(u.Email))

				if _, err := datastore.Put(c, k, u); err != nil {
					log.Error("Could not Put User because %v", err, c)
					return err
				}

				return nil
			}

			return errors.New("Invalid type, required: *User")
		}))
