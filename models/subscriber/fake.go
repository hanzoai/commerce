package subscriber

import (
	"strings"

	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Subscriber {
	s := New(db)
	s.UserId = userId
	s.MailingListId = fake.RandSeq(10, []rune("abcdefghijklmnopqrstuvwxyz"))
	s.Email = strings.ToLower(fake.EmailAddress())
	s.Unsubscribed = false
	return s
}
