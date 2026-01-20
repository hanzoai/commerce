package webhook

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore) *Webhook {
	s := New(db)
	s.Url = fake.Url()
	s.Live = false
	s.All = true
	s.Enabled = true
	return s
}
