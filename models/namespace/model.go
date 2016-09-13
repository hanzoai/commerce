package namespace

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/namespace/consts"
)

func (n Namespace) Kind() string {
	return "namespace"
}

func (n *Namespace) Init(db *datastore.Datastore) {
	n.Model.Init(db, n)
	n.SetNamespace(consts.Namespace)
	n.Ancestor = db.NewKey(n.Kind(), "", consts.RootKey, nil)
	n.UseStringKey = true
}

func New(db *datastore.Datastore) *Namespace {
	n := new(Namespace)
	n.Init(db)
	return n
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
