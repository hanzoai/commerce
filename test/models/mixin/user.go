package test

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
)

type User struct {
	mixin.Model
	mixin.AccessToken

	Name string
}

func (u *User) Kind() string {
	return "user2"
}

func newUser(db *datastore.Datastore) *User {
	u := new(User)
	u.Model = mixin.Model{Db: db, Entity: u}
	u.AccessToken = mixin.AccessToken{Model: u}
	return u
}
