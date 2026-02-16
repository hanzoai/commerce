package applicationmethod

import "github.com/hanzoai/commerce/datastore"

var kind = "applicationmethod"

func (a ApplicationMethod) Kind() string {
	return kind
}

func (a *ApplicationMethod) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *ApplicationMethod) Defaults() {
}

func New(db *datastore.Datastore) *ApplicationMethod {
	a := new(ApplicationMethod)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
