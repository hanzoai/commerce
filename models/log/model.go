package log

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (l Log) Kind() string {
	return "log"
}

func (l *Log) Init(db *datastore.Datastore) {
	l.Model.Init(db, l)
}

func New(db *datastore.Datastore) *Log {
	l := new(Log)
	l.Init(db)
	return l
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
