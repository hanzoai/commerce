package customergroupmembership

import "github.com/hanzoai/commerce/datastore"

var kind = "customergroupmembership"

func (m CustomerGroupMembership) Kind() string {
	return kind
}

func (m *CustomerGroupMembership) Init(db *datastore.Datastore) {
	m.Model.Init(db, m)
}

func (m *CustomerGroupMembership) Defaults() {
}

func New(db *datastore.Datastore) *CustomerGroupMembership {
	m := new(CustomerGroupMembership)
	m.Init(db)
	m.Defaults()
	return m
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
