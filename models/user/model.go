package user

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (u User) Kind() string {
	return "user"
}

func (u *User) Init(db *datastore.Datastore) {
	u.Counter = mixin.Counter{Entity: u}
	u.Model = mixin.Model{Db: db, Entity: u}
}

func (u *User) Defaults() {
	u.Metadata = make(Map)
	u.History = make([]Event, 0)
}

func New(db *datastore.Datastore) *User {
	u := new(User)
	u.Init(db)
	u.Defaults()
	return u
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
