package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/order", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
	ord *order.Order
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	ord = fixtures.Order(c).(*order.Order)
	fixtures.Coupon(c)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Order", func() {
	It("Should UpdateAndTally", func() {
		ord.CouponCodes = []string{}
		err := ord.UpdateAndTally(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(ord.Subtotal).To(Equal(currency.Cents(50000)))
		Expect(ord.Total).To(Equal(currency.Cents(50000)))
	})

	It("Should UpdateAndTally With Coupon", func() {
		ord.CouponCodes = []string{"such-coupon"}
		err := ord.UpdateAndTally(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(ord.Subtotal).To(Equal(currency.Cents(49500)))
		Expect(ord.Total).To(Equal(currency.Cents(49500)))
	})

	It("Should UpdateAndTally And Dedupe Coupons", func() {
		ord.CouponCodes = []string{"such-coupon", "such-coupon"}
		err := ord.UpdateAndTally(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(ord.CouponCodes)).To(Equal(1))
		Expect(ord.CouponCodes[0]).To(Equal("such-coupon"))
		Expect(ord.Subtotal).To(Equal(currency.Cents(49500)))
		Expect(ord.Total).To(Equal(currency.Cents(49500)))
	})

	It("Should UpdateAndTally Only Applicable Coupons", func() {
		ord2 := order.New(ord.Db)
		ord2.CouponCodes = []string{"sad-coupon"}
		ord2.Items = []lineitem.LineItem{lineitem.LineItem{ProductSlug: "doge-shirt", Quantity: 1}}
		err := ord2.UpdateAndTally(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(ord2.Subtotal).To(Equal(currency.Cents(2000)))
		Expect(ord2.Total).To(Equal(currency.Cents(2000)))

		ord2 = order.New(ord.Db)
		ord2.CouponCodes = []string{"sad-coupon"}
		ord2.Items = []lineitem.LineItem{lineitem.LineItem{ProductSlug: "sad-keanu-shirt", Quantity: 1}}
		err = ord2.UpdateAndTally(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(ord2.Subtotal).To(Equal(currency.Cents(2000)))
		Expect(ord2.Total).To(Equal(currency.Cents(2000)))
	})

	It("Should Serialize Line Items", func() {
		ord2 := order.New(ord.Db)
		ord2.CouponCodes = []string{"sad-coupon"}
		ord2.Items = []lineitem.LineItem{lineitem.LineItem{ProductSlug: "doge-shirt", ProductName: "Doge Shirt", Quantity: 1}}

		memo := ord2.DescriptionLong()
		Expect(memo).To(Equal("Doge Shirt (doge-shirt) x 1\n"))
	})
})
