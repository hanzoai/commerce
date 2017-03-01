package log_

import "hanzo.io/datastore"

var kind = "log"

func (l Log) Kind() string {
	return kind
}

func (l *Log) Init(db *datastore.Datastore) {
	l.Model.Init(db, l)
}

func (l *Log) Defaults() {
	l.Enabled = true
}

func New(db *datastore.Datastore) *Log {
	l := new(Log)
	l.Init(db)
	l.Defaults()
	return l
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
