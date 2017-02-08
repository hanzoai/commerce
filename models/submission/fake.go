package submission

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Submission {
	s := New(db)
	s.UserId = userId
	s.Email = fake.EmailAddress()
	return s
}
