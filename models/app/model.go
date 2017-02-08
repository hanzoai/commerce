package app

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (a App) Kind() string {
	return "app"
}

func (a *App) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func New(db *datastore.Datastore) *App {
	a := new(App)
	a.Init(db)
	return a
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
