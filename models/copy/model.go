package copy

import "github.com/hanzoai/commerce/datastore"

var kind = "copy"

func (a Copy) Kind() string {
	return kind
}

func (a *Copy) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Copy) Defaults() {
	a.Type = ContentType
}

func New(db *datastore.Datastore) *Copy {
	a := new(Copy)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
