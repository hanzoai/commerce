package transaction

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (t Transaction) Kind() string {
	return "transaction"
}

func (t *Transaction) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func New(db *datastore.Datastore) *Transaction {
	t := new(Transaction)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
