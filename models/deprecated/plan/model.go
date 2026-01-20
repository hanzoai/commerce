package plan

import "github.com/hanzoai/commerce/datastore"

var kind = "plan"

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Plan) Defaults() {
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
