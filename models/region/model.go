package region

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "region"

func (r Region) Kind() string {
	return kind
}

func (r *Region) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Region) Defaults() {
	r.Countries = make([]Country, 0)
	r.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Region {
	r := new(Region)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
