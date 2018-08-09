package subscriber

import (
	"strings"

	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Subscriber {
	s := New(db)
	s.UserId = userId
	s.FormId = fake.RandSeq(10, []rune("abcdefghijklmnopqrstuvwxyz"))
	s.Email = strings.ToLower(fake.EmailAddress())
	s.Unsubscribed = false
	return s
}
