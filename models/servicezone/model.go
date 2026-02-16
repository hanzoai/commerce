package servicezone

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "servicezone"

func (s ServiceZone) Kind() string {
	return kind
}

func (s *ServiceZone) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *ServiceZone) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *ServiceZone {
	s := new(ServiceZone)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
