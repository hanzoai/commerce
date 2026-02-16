package role

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "role"

func (r Role) Kind() string {
	return kind
}

func (r *Role) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Role) Defaults() {
	r.Permissions = make([]string, 0)
	r.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Role {
	r := new(Role)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
