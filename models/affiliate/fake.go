package affiliate

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/country"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Affiliate {
	aff := New(db)
	aff.Name = fake.FullName()
	aff.UserId = userId
	aff.Company = fake.Company()
	aff.Country = country.Fake()
	aff.TaxId = fake.RandSeq(9, []rune("0123456789"))
	aff.Commission = commission.Fake()
	aff.CouponId = "TEST-COUPON"
	return aff
}
