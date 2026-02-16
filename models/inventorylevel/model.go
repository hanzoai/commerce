package inventorylevel

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "inventorylevel"

func (l InventoryLevel) Kind() string {
	return kind
}

func (l *InventoryLevel) Init(db *datastore.Datastore) {
	l.Model.Init(db, l)
}

func (l *InventoryLevel) Defaults() {
	l.Metadata = make(Map)
}

func New(db *datastore.Datastore) *InventoryLevel {
	l := new(InventoryLevel)
	l.Init(db)
	l.Defaults()
	return l
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
