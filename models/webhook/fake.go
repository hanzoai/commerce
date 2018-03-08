package webhook

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *Webhook {
	s := New(db)
	s.Url = fake.Url()
	s.Live = false
	s.All = true
	s.Enabled = true
	return s
}
