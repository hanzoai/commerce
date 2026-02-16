package stocklocation

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "stocklocation"

func (s StockLocation) Kind() string {
	return kind
}

func (s *StockLocation) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *StockLocation) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *StockLocation {
	s := new(StockLocation)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
