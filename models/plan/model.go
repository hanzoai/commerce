package plan

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"

	. "hanzo.io/models"
)

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Plan) Defaults() {
	p.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
