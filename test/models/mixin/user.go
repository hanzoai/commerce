package test

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
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
	u.AccessToken = mixin.AccessToken{Entity: u}
	return u
}

func (u *User) Validator() *val.Validator {
	return val.New(u)
}
