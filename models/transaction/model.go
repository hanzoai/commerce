package transaction

import "hanzo.io/datastore"

var kind = "transaction"

func (t Transaction) Kind() string {
	return kind
}

func (t *Transaction) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *Transaction) Defaults() {
	t.Parent = t.Db.NewKey("synckey", "", 1, nil)
}

func New(db *datastore.Datastore) *Transaction {
	t := new(Transaction)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
