package referrer

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (r Referrer) Kind() string {
	return "referrer"
}

func (r *Referrer) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Referrer) Defaults() {
	r.Program.Triggers = make([]int, 0)
	r.Program.Actions = make([]Action, 0)
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
