package trigger

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (t Trigger) Kind() string {
	return "trigger"
}

func (t *Trigger) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func New(db *datastore.Datastore) *Trigger {
	t := new(Trigger)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
