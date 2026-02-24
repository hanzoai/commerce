package inventory

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "inventoryitem"

func (r InventoryItem) Kind() string {
	return kind
}

func (r *InventoryItem) Init(db *datastore.Datastore) {
	r.BaseModel.Init(db, r)
}

func (r *InventoryItem) Defaults() {
	r.Metadata = make(Map)
}

func New(db *datastore.Datastore) *InventoryItem {
	r := new(InventoryItem)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
