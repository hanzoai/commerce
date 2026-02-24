package taxraterule

import "github.com/hanzoai/commerce/datastore"

var kind = "taxraterule"

func (t TaxRateRule) Kind() string {
	return kind
}

func (t *TaxRateRule) Init(db *datastore.Datastore) {
	t.BaseModel.Init(db, t)
}

func (t *TaxRateRule) Defaults() {
}

func New(db *datastore.Datastore) *TaxRateRule {
	t := new(TaxRateRule)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
