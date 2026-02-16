package saleschannel

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "saleschannel"

func (s SalesChannel) Kind() string {
	return kind
}

func (s *SalesChannel) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *SalesChannel) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *SalesChannel {
	s := new(SalesChannel)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
