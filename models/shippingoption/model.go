package shippingoption

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "shippingoption"

func (s ShippingOption) Kind() string {
	return kind
}

func (s *ShippingOption) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *ShippingOption) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *ShippingOption {
	s := new(ShippingOption)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
