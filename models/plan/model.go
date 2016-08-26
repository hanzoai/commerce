package plan

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
