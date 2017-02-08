package transfer

import "hanzo.io/datastore"

var kind = "transfer"

func (t Transfer) Kind() string {
	return kind
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
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
