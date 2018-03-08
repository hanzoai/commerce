package coupon

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *Redemption {
	r := New(db)
	r.Code = fake.Word()
	return r
}
