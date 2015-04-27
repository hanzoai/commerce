package namespace

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
)

type Namespace struct {
	mixin.Model

	IntId    int64
	StringId string
}

func New(db *datastore.Datastore) *Namespace {
	n := new(Namespace)
	n.Model = mixin.Model{Db: db, Entity: n}
	return n
}

func (c Namespace) Kind() string {
	return "namespace"
}

func (c *Namespace) Validator() *val.Validator {
	return val.New(c)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
