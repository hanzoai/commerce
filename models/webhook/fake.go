package webhook

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore) *Webhook {
	s := New(db)
	s.Url = "http://www." + fake.Word() + ".com/hook/" + fake.Word()
	s.Live = false
	s.All = true
	s.Enabled = true
	return s
}
