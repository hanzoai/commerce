package networktoken

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/types"
)

var kind = "network-token"

func (nt NetworkToken) Kind() string {
	return kind
}

func (nt *NetworkToken) Init(db *datastore.Datastore) {
	nt.BaseModel.Init(db, nt)
}

func (nt *NetworkToken) Defaults() {
	nt.Parent = nt.Db.NewKey("synckey", "", 1, nil)
	if nt.Status == "" {
		nt.Status = Active
	}
	if nt.Metadata == nil {
		nt.Metadata = make(Map)
	}
}

func New(db *datastore.Datastore) *NetworkToken {
	nt := new(NetworkToken)
	nt.Init(db)
	nt.Defaults()
	return nt
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
