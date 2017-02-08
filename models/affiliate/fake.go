package affiliate

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/commission"
	"hanzo.io/models/types/country"
	"hanzo.io/util/fake"
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
