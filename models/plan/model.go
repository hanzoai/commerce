package plan

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func New(db *datastore.Datastore) *Plan {
	return new(Plan).New(db).(*Plan)
}

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Init(db *datastore.Datastore) {
	p.Model = mixin.Model{Db: db, Entity: p}
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
