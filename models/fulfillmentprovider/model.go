package fulfillmentprovider

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "fulfillmentprovider"

func (p FulfillmentProvider) Kind() string {
	return kind
}

func (p *FulfillmentProvider) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *FulfillmentProvider) Defaults() {
	p.Metadata = make(Map)
}

func New(db *datastore.Datastore) *FulfillmentProvider {
	p := new(FulfillmentProvider)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
