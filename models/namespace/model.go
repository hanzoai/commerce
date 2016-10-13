package namespace

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace/consts"
)

var kind = "namespace"

func (n Namespace) Kind() string {
	return kind
}

func (n *Namespace) Init(db *datastore.Datastore) {
	n.Model.Init(db, n)
	n.SetNamespace(consts.Namespace)
	n.Parent = db.NewKey(n.Kind(), "", consts.RootKey, nil)
	n.UseStringKey = true
}

func (n *Namespace) Defaults() {
}

func New(db *datastore.Datastore) *Namespace {
	n := new(Namespace)
	n.Init(db)
	n.Defaults()
	return n
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
