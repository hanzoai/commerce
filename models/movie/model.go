package movie

import (
	"hanzo.io/datastore"
)

var kind = "movie"

func (m Movie) Kind() string {
	return kind
}

func (m *Movie) Init(db *datastore.Datastore) {
	m.Model.Init(db, m)
}

func (m *Movie) Defaults() {
}

func New(db *datastore.Datastore) *Movie {
	m := new(Movie)
	m.Init(db)
	m.Defaults()
	return m
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
