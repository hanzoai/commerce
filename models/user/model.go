package user

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "user"

func (u User) Kind() string {
	return kind
}

func (u *User) Init(db *datastore.Datastore) {
	u.BaseModel.Init(db, u)
}

func (u *User) Defaults() {
	if u != nil {
		u.Metadata = make(map[string]interface{})
	}
	u.History = make([]Event, 0)
	u.Organizations = make([]string, 0)
	u.KYC.Documents = make([]string, 0)
	u.KYC.Status = KYCStatusInitiated
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
