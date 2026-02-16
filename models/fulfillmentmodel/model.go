package fulfillmentmodel

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "fulfillment"

func (f Fulfillment) Kind() string {
	return kind
}

func (f *Fulfillment) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func (f *Fulfillment) Defaults() {
	f.Items = make([]FulfillmentItem, 0)
	f.Labels = make([]FulfillmentLabel, 0)
	f.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Fulfillment {
	f := new(Fulfillment)
	f.Init(db)
	f.Defaults()
	return f
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
