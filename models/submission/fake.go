package submission

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Submission {
	s := New(db)
	s.UserId = userId
	s.Email = fake.EmailAddress()
	return s
}
