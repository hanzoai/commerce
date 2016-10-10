package partner

import "crowdstart.com/datastore"

var kind = "partner"

func (p Partner) Kind() string {
	return kind
}

func (p *Partner) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func New(db *datastore.Datastore) *Partner {
	p := new(Partner)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
