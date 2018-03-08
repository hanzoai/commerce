package app

import (
	"hanzo.io/datastore"
)

var kind = "app"

func (a App) Kind() string {
	return kind
}

func (a *App) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func New(db *datastore.Datastore) *App {
	a := new(App)
	a.Init(db)
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
