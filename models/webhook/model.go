package webhook

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (w Webhook) Kind() string {
	return "webhook"
}

func (w *Webhook) Init(db *datastore.Datastore) {
	w.Model.Init(db, w)
}

func New(db *datastore.Datastore) *Webhook {
	w := new(Webhook)
	w.Init(db)
	return w
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
