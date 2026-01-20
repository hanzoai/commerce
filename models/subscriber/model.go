package subscriber

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "subscriber"

func (s Subscriber) Kind() string {
	return kind
}

func (s *Subscriber) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)

	s.Tags = []string{}
}

func (s *Subscriber) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Subscriber {
	s := new(Subscriber)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
