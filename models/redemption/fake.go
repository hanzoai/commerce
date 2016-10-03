package coupon

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore) *Redemption {
	r := New(db)
	r.Code = fake.Word()
	return r
}
