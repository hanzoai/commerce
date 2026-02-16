package notification

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "notification"

func (n Notification) Kind() string {
	return kind
}

func (n *Notification) Init(db *datastore.Datastore) {
	n.Model.Init(db, n)
}

func (n *Notification) Defaults() {
	n.Status = Pending
	n.Data = make(Map)
	n.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Notification {
	n := new(Notification)
	n.Init(db)
	n.Defaults()
	return n
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
