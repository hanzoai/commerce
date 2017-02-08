package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/payment"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/payment", t)
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

var _ = Describe("Payment", func() {
	Context("New payment", func() {
		var pay *payment.Payment

		Before(func() {
			pay = payment.Fake(db)
		})

		It("should save new payments", func() {
			pay.MustPut()
		})

		It("should correctly persist metadata", func() {
			pay.Metadata["a"] = 1
			pay.Metadata["orderId"] = "some-order"
			pay.MustPut()
			pay2 := payment.New(db)
			pay2.MustGet(pay.Key())
			Expect(pay2.Metadata["a"]).To(Equal(float64(1)))
			Expect(pay2.Metadata["orderId"]).To(Equal("some-order"))
		})
	})
})
