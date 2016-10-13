package webhook

import "crowdstart.com/datastore"

var kind = "webhook"

func (w Webhook) Kind() string {
	return kind
}

func (w *Webhook) Init(db *datastore.Datastore) {
	w.Model.Init(db, w)
}

func (w *Webhook) Defaults() {
	w.Events = make(Events)
}

func New(db *datastore.Datastore) *Webhook {
	w := new(Webhook)
	w.Init(db)
	w.Defaults()
	return w
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
