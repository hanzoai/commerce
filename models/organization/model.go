package organization

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (o Organization) Kind() string {
	return "organization"
}

func (o *Organization) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
	o.AccessToken.Init(o)
}

func (o *Organization) Defaults() {
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)
}

func New(db *datastore.Datastore) *Organization {
	r := new(Organization)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
