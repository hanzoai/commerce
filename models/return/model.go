package return_

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/lineitem"

	. "github.com/hanzoai/commerce/types"
)

var kind = "return"

func (r Return) Kind() string {
	return kind
}

func (r *Return) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Return) Defaults() {
	r.Items = make([]lineitem.LineItem, 0)
	r.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Return {
	r := new(Return)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
