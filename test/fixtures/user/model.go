package user

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (u User) Kind() string {
	return "user"
}

func (u *User) Init(db *datastore.Datastore) {
	u.Model.Init(db, u)
}

func New(db *datastore.Datastore) *User {
	u := new(User)
	u.Init(db)
	return u
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
