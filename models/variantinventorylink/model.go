package variantinventorylink

import "github.com/hanzoai/commerce/datastore"

var kind = "variantinventorylink"

func (l VariantInventoryLink) Kind() string {
	return kind
}

func (l *VariantInventoryLink) Init(db *datastore.Datastore) {
	l.BaseModel.Init(db, l)
}

func (l *VariantInventoryLink) Defaults() {
}

func New(db *datastore.Datastore) *VariantInventoryLink {
	l := new(VariantInventoryLink)
	l.Init(db)
	l.Defaults()
	return l
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
