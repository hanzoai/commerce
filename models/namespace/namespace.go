package namespace

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/constants"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
)

type Namespace struct {
	mixin.Model

	IntId int64
	Name  string
}

func New(db *datastore.Datastore) *Namespace {
	n := new(Namespace)
	n.Model = mixin.Model{Db: db, Entity: n}
	n.SetNamespace(constants.NamespaceNamespace)
	n.Parent = db.NewKey(n.Kind(), "", 1, nil)
	n.UseStringKey = true
	return n
}

func (n Namespace) Kind() string {
	return "namespace"
}

func (n *Namespace) Validator() *val.Validator {
	return val.New(n)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}

func (n *Namespace) Exists(name string) (ok bool, err error) {
	n.RunInTransaction(func() error {
		_, ok, err = n.Model.KeyExists(name)
		return err
	})

	return ok, err
}
