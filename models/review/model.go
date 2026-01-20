package review

import "github.com/hanzoai/commerce/datastore"

var kind = "review"

func (r Review) Kind() string {
	return kind
}

func (r *Review) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Review) Defaults() {
	if r != nil {
		r.Metadata = make(map[string]interface{})
	}
}

func New(db *datastore.Datastore) *Review {
	r := new(Review)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
