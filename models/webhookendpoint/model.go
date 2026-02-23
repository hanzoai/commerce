package webhookendpoint

import "github.com/hanzoai/commerce/datastore"

var kind = "webhook-endpoint"

func (w WebhookEndpoint) Kind() string {
	return kind
}

func (w *WebhookEndpoint) Init(db *datastore.Datastore) {
	w.Model.Init(db, w)
}

func (w *WebhookEndpoint) Defaults() {
	w.Parent = w.Db.NewKey("synckey", "", 1, nil)
	if w.Status == "" {
		w.Status = "enabled"
	}
}

func New(db *datastore.Datastore) *WebhookEndpoint {
	w := new(WebhookEndpoint)
	w.Init(db)
	w.Defaults()
	return w
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
