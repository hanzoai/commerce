package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/coupon"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/coupon", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Coupon", func() {
	Context("GetById", func() {
		var cpn *coupon.Coupon

		Before(func() {
			cpn = coupon.Fake(db)
			cpn.MustCreate()
		})

		It("should retrieve coupon from datastore by code", func() {
			cpn2 := coupon.New(db)
			cpn2.MustGetById(cpn.Code())
			Expect(cpn2.Code()).To(Equal(cpn2.Code()))
		})
	})
})
