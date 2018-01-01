package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/store"
	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/georate"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/rand"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/order", t)
}

var (
	ctx   ae.Context
	db    *datastore.Datastore
	ord   *order.Order
	stor  *store.Store
	stor2 *store.Store
	stor3 *store.Store
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	ord = fixtures.Order(c).(*order.Order)

	stor = store.New(ord.Db)
	stor.MustCreate()

	stor2 = store.New(ord.Db)
	stor2.MustCreate()

	sr, err := stor.GetShippingRates()
	Expect(err).NotTo(HaveOccurred())

	sr.GeoRates = append(sr.GeoRates, shippingrates.GeoRate{
		georate.New(
			"",
			"",
			"",
			"",
			0.1,
			499,
		),
		"SHIPPING",
	})
	sr.MustUpdate()

	tr, err := stor.GetTaxRates()
	Expect(err).NotTo(HaveOccurred())
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		georate.New(
			"US",
			"KS",
			"",
			"66212",
			0.0885,
			1,
		),
		false,
		"TEST TAX",
	})
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		georate.New(
			"US",
			"KS",
			"",
			"",
			0.065,
			1,
		),
		false,
		"TEST TAX",
	})
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		georate.New(
			"GB",
			"",
			"",
			"",
			0.2,
			1,
		),
		false,
		"VAT",
	})
	tr.MustUpdate()

	fixtures.Coupon(c)

	stor2 = store.New(ord.Db)
	stor2.MustCreate()

	sr2, err := stor2.GetShippingRates()
	Expect(err).NotTo(HaveOccurred())

	sr2.MustDelete()

	tr2, err := stor2.GetTaxRates()
	Expect(err).NotTo(HaveOccurred())

	stor3 = store.New(ord.Db)
	stor3.MustCreate()
	stor3.Currency = currency.ETH

	price := currency.Cents(1234)

	stor3.Listings = make(map[string]store.Listing)
	stor3.Listings[ord.Items[0].ProductId] = store.Listing{Price: &price}

	tr2.MustDelete()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Order", func() {
	Context("Storeless (Deprecated)", func() {
		// Make sure out of order execution works because tax is left alone in
		// deprecated flow
		BeforeEach(func() {
			// Scramble currency values so we know they are being replaced
			for i, _ := range ord.Coupons {
				ord.Coupons[i].Amount = rand.Int()
			}

			for i, _ := range ord.Items {
				ord.Items[i].Price = currency.Cents(rand.Int64())
			}

			ord.LineTotal = currency.Cents(rand.Int64())
			ord.Discount = currency.Cents(rand.Int64())
			ord.Subtotal = currency.Cents(rand.Int64())
			ord.Tax = 0      //currency.Cents(rand.Int64())
			ord.Shipping = 0 //currency.Cents(rand.Int64())
			ord.Total = currency.Cents(rand.Int64())
			ord.TokenSaleId = ""
			ord.WalletId = ""
			ord.WalletPassphrase = ""
			ord.Mode = order.DefaultMode
		})

		It("Should UpdateAndTally", func() {
			ord.CouponCodes = []string{}
			err := ord.UpdateAndTally(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(currency.Cents(50000)))
			Expect(ord.Total).To(Equal(currency.Cents(50000)))
		})

		It("Should UpdateAndTally With No Tax or ShippingRates without crashing", func() {
			sr, _ := stor2.GetShippingRates()
			Expect(sr).To(BeNil())

			tr, _ := stor2.GetTaxRates()
			Expect(tr).To(BeNil())

			ord.CouponCodes = []string{}
			err := ord.UpdateAndTally(stor2)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(currency.Cents(50000)))

			tax := currency.Cents(0)
			shipping := currency.Cents(0)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
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

	Context("Store", func() {
		BeforeEach(func() {
			// Scramble currency values so we know they are being replaced
			for i, _ := range ord.Coupons {
				ord.Coupons[i].Amount = rand.Int()
			}

			for i, _ := range ord.Items {
				ord.Items[i].Price = currency.Cents(rand.Int64())
			}

			ord.LineTotal = currency.Cents(rand.Int64())
			ord.Discount = currency.Cents(rand.Int64())
			ord.Subtotal = currency.Cents(rand.Int64())
			ord.Tax = currency.Cents(rand.Int64())
			ord.Shipping = currency.Cents(rand.Int64())
			ord.Total = currency.Cents(rand.Int64())
			ord.TokenSaleId = ""
			ord.WalletId = ""
			ord.WalletPassphrase = ""
			ord.Mode = order.DefaultMode
		})

		It("Should UpdateAndTally", func() {
			ord.CouponCodes = []string{}
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(currency.Cents(50000)))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally Price Overrides", func() {
			ord.CouponCodes = []string{}
			err := ord.UpdateAndTally(stor3)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(currency.Cents(24680)))
			Expect(ord.Currency).To(Equal(currency.ETH))

			Expect(ord.Total).To(Equal(ord.Subtotal))

		})

		It("Should UpdateAndTally with Provided Subtotal for Contributions", func() {
			ord.CouponCodes = []string{}
			ord.Mode = order.ContributionMode
			subTotal := ord.Subtotal
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(subTotal))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally with Provided Subtotal for Deposit", func() {
			ord.CouponCodes = []string{}
			ord.Mode = order.ContributionMode
			subTotal := ord.Subtotal
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(subTotal))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally with Provided Subtotal for TokenSales", func() {
			ord.CouponCodes = []string{}
			ord.TokenSaleId = "1234"
			subTotal := ord.Subtotal
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord.Subtotal).To(Equal(subTotal))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally With Coupon", func() {
			ord.CouponCodes = []string{"such-coupon"}
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())

			Expect(ord.Subtotal).To(Equal(currency.Cents(49500)))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally And Dedupe Coupons", func() {
			ord.CouponCodes = []string{"such-coupon", "such-coupon"}
			err := ord.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ord.CouponCodes)).To(Equal(1))
			Expect(ord.CouponCodes[0]).To(Equal("such-coupon"))

			Expect(ord.Subtotal).To(Equal(currency.Cents(49500)))

			tax := 1 + currency.Cents(float64(ord.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord.Subtotal)*0.1)

			Expect(ord.Tax).To(Equal(tax))
			Expect(ord.Shipping).To(Equal(shipping))
			Expect(ord.Total).To(Equal(ord.Subtotal + tax + shipping))
		})

		It("Should UpdateAndTally Only Applicable Coupons", func() {
			ord2 := order.New(ord.Db)
			ord2.CouponCodes = []string{"sad-coupon"}
			ord2.Items = []lineitem.LineItem{lineitem.LineItem{ProductSlug: "doge-shirt", Quantity: 1}}
			ord2.ShippingAddress.Country = "us"
			ord2.ShippingAddress.State = "ks"
			ord2.ShippingAddress.PostalCode = "66212"

			err := ord2.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord2.Subtotal).To(Equal(currency.Cents(2000)))

			tax := 1 + currency.Cents(float64(ord2.Subtotal)*0.0885)
			shipping := 499 + currency.Cents(float64(ord2.Subtotal)*0.1)

			Expect(ord2.Tax).To(Equal(tax))
			Expect(ord2.Shipping).To(Equal(shipping))
			Expect(ord2.Total).To(Equal(ord2.Subtotal + tax + shipping))

			ord2 = order.New(ord.Db)
			ord2.CouponCodes = []string{"sad-coupon"}
			ord2.Items = []lineitem.LineItem{lineitem.LineItem{ProductSlug: "sad-keanu-shirt", Quantity: 1}}
			ord2.ShippingAddress.Country = "us"
			ord2.ShippingAddress.State = "ks"
			ord2.ShippingAddress.PostalCode = "66212"

			err = ord2.UpdateAndTally(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(ord2.Subtotal).To(Equal(currency.Cents(2000)))

			tax = 1 + currency.Cents(float64(ord2.Subtotal)*0.0885)
			shipping = 499 + currency.Cents(float64(ord2.Subtotal)*0.1)

			Expect(ord2.Tax).To(Equal(tax))
			Expect(ord2.Shipping).To(Equal(shipping))
			Expect(ord2.Total).To(Equal(ord2.Subtotal + tax + shipping))
		})
	})
})
