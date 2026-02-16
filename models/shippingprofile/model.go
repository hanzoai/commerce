package shippingprofile

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "shippingprofile"

func (s ShippingProfile) Kind() string {
	return kind
}

func (s *ShippingProfile) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *ShippingProfile) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *ShippingProfile {
	s := new(ShippingProfile)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
