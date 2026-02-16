package fulfillmentset

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "fulfillmentset"

func (f FulfillmentSet) Kind() string {
	return kind
}

func (f *FulfillmentSet) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func (f *FulfillmentSet) Defaults() {
	f.Metadata = make(Map)
}

func New(db *datastore.Datastore) *FulfillmentSet {
	f := new(FulfillmentSet)
	f.Init(db)
	f.Defaults()
	return f
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
