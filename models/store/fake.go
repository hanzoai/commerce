package store

import (
	"crowdstart.com/datastore"
	. "crowdstart.com/models"
)

func Fake(db *datastore.Datastore) *Store {
	s := New(db)
	return s
}
