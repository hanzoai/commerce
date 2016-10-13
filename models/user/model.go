package user

import (
	"crowdstart.com/datastore"

	. "crowdstart.com/models"
)

var kind = "user"

func (u User) Kind() string {
	return kind
}

func (u *User) Init(db *datastore.Datastore) {
	u.Counter.Init(u)
	u.Model.Init(db, u)
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

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
