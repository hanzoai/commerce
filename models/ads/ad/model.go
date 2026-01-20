package ad

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/models/ads"
)

var kind = "ad"

func (a Ad) Kind() string {
	return kind
}

func (a *Ad) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Ad) Defaults() {
	a.Status = PendingStatus
}

func New(db *datastore.Datastore) *Ad {
	a := new(Ad)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
