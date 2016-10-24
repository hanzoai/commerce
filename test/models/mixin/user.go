package test

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
)

type User struct {
	mixin.Model
	mixin.AccessToken

	Email string
	Name  string

	Friends []string
}

// We shadow the real user's kind because kinds must be defined in our hashid
// map or none of this works.
func (u User) Kind() string {
	return "user"
}

func (u *User) Defaults() {
	u.Friends = make([]string, 0)
}

func (u *User) Init(db *datastore.Datastore) {
	u.Model.Init(db, u)
	u.AccessToken = mixin.AccessToken{Entity: u}
}

func (u *User) Document() mixin.Document {
	return nil
}

func newUser(db *datastore.Datastore) *User {
	u := new(User)
	u.Init(db)
	u.Defaults()
	return u
}

func (u *User) Validator() *val.Validator {
	return val.New()
}
