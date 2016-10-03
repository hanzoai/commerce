package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore) *Site {
	s := New(db)
	s.Domain = fake.Word()
	s.Name = fake.Company()
	s.Url = "https://" + s.Domain + ".com"
	s.Netlify_ = netlify.Site{
		Name:              s.Name,
		Domain:            s.Domain,
		Password:          fake.RandSeq(10, []rune("abcdefghijklmnopqrstuvwxyz")),
		NotificationEmail: fake.EmailAddress(),
	}
	return s

}
