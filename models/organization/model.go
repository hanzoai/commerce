package organization

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (o Organization) Kind() string {
	return "organization"
}

func (o *Organization) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
}

func New(db *datastore.Datastore) *Organization {
	r := new(Organization)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
