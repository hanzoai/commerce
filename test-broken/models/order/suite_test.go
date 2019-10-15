package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/product"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/store"
	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/georate"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/types"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/order", t)
}

var (
	ctx     ae.Context
	db      *datastore.Datastore
	ord     *order.Order
	stor    *store.Store
	stor2   *store.Store
	stor3   *store.Store
	subProd *product.Product
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	ord = fixtures.Order(c).(*order.Order)

	// Add a subscription item
	subProd = product.Fake(ord.Db)
	subProd.IsSubscribeable = true
	subProd.Interval = Monthly
	subProd.IntervalCount = 1
	subProd.TrialPeriodDays = 1
	subProd.MustPut()

	ord.Items = append(ord.Items, lineitem.LineItem{
		ProductId: subProd.Id(),
		Quantity:  1,
	})

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
