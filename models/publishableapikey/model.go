package publishableapikey

import "github.com/hanzoai/commerce/datastore"

var kind = "publishableapikey"

func (k PublishableApiKey) Kind() string {
	return kind
}

func (k *PublishableApiKey) Init(db *datastore.Datastore) {
	k.Model.Init(db, k)
}

func (k *PublishableApiKey) Defaults() {
}

func New(db *datastore.Datastore) *PublishableApiKey {
	k := new(PublishableApiKey)
	k.Init(db)
	k.Defaults()
	return k
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
