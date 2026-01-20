package submission

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Submission {
	s := New(db)
	s.UserId = userId
	s.Email = fake.EmailAddress()
	return s
}
