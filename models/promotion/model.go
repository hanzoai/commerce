package promotion

import "github.com/hanzoai/commerce/datastore"

var kind = "promotion"

func (p Promotion) Kind() string {
	return kind
}

func (p *Promotion) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Promotion) Defaults() {
	if p.Status == "" {
		p.Status = "draft"
	}
}

func New(db *datastore.Datastore) *Promotion {
	p := new(Promotion)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
