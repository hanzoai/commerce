package taxregion

import "github.com/hanzoai/commerce/datastore"

var kind = "taxregion"

func (t TaxRegion) Kind() string {
	return kind
}

func (t *TaxRegion) Init(db *datastore.Datastore) {
	t.BaseModel.Init(db, t)
}

func (t *TaxRegion) Defaults() {
}

func New(db *datastore.Datastore) *TaxRegion {
	t := new(TaxRegion)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
