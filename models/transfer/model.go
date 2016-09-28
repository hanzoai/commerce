package transfer

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (t Transfer) Kind() string {
	return "transfer"
}

func (t *Transfer) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *Transfer) Defaults() {
	t.Status = Pending
}

func New(db *datastore.Datastore) *Transfer {
	t := new(Transfer)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
