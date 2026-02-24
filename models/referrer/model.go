package referrer

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/types"
)

var kind = "referrer"

func (r Referrer) Kind() string {
	return kind
}

func (r *Referrer) Init(db *datastore.Datastore) {
	r.BaseModel.Init(db, r)
}

func (r *Referrer) Defaults() {
	r.State = make(Map)
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
