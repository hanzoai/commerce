package transaction

import "crowdstart.com/datastore"

var kind = "transaction"

func (t Transaction) Kind() string {
	return kind
}

func (t *Transaction) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func New(db *datastore.Datastore) *Transaction {
	t := new(Transaction)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
