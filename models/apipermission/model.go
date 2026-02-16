package apipermission

import "github.com/hanzoai/commerce/datastore"

var kind = "apipermission"

func (p ApiPermission) Kind() string {
	return kind
}

func (p *ApiPermission) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *ApiPermission) Defaults() {
}

func New(db *datastore.Datastore) *ApiPermission {
	p := new(ApiPermission)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
