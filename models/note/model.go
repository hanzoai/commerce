package note

import "github.com/hanzoai/commerce/datastore"

var kind = "note"

func (n Note) Kind() string {
	return kind
}

func (n *Note) Init(db *datastore.Datastore) {
	n.Model.Init(db, n)
}

func (n *Note) Defaults() {
	n.Enabled = true
}

func New(db *datastore.Datastore) *Note {
	n := new(Note)
	n.Init(db)
	n.Defaults()
	return n
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
