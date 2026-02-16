package reservation

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "reservation"

func (r ReservationItem) Kind() string {
	return kind
}

func (r *ReservationItem) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *ReservationItem) Defaults() {
	r.Metadata = make(Map)
}

func New(db *datastore.Datastore) *ReservationItem {
	r := new(ReservationItem)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
