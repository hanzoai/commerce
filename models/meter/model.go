package meter

import "github.com/hanzoai/commerce/datastore"

var kind = "meter"

func (m Meter) Kind() string {
	return kind
}

func (m *Meter) Init(db *datastore.Datastore) {
	m.Model.Init(db, m)
}

func (m *Meter) Defaults() {
	m.Parent = m.Db.NewKey("synckey", "", 1, nil)
}

func New(db *datastore.Datastore) *Meter {
	m := new(Meter)
	m.Init(db)
	m.Defaults()
	return m
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
