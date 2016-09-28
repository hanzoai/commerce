package partner

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (p Partner) Kind() string {
	return "partner"
}

func (p *Partner) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func New(db *datastore.Datastore) *Partner {
	p := new(Partner)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
