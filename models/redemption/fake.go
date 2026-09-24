package coupon

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore) *Redemption {
	r := New(db)
	r.Code = fake.Word()
	return r
}
