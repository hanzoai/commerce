package subscriber

import (
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Subscriber {
	s := New(db)
	s.UserId = userId
	s.FormId = fake.RandSeq(10, []rune("abcdefghijklmnopqrstuvwxyz"))
	s.Email = strings.ToLower(fake.EmailAddress())
	s.Unsubscribed = false
	return s
}
