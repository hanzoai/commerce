package taxprovider

import "github.com/hanzoai/commerce/datastore"

var kind = "taxprovider"

func (t TaxProvider) Kind() string {
	return kind
}

func (t *TaxProvider) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *TaxProvider) Defaults() {
	t.IsEnabled = true
}

func New(db *datastore.Datastore) *TaxProvider {
	t := new(TaxProvider)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
