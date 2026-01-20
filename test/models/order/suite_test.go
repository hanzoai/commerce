package test

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/shippingrates"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/taxrates"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/georate"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/types"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
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
		GeoRate: georate.New(
			"",
			"",
			"",
			"",
			currency.Cents(10),
			currency.Cents(0),
			0,
			currency.Cents(100),
		),
		ShippingName: "SHIPPING",
	})
	sr.MustUpdate()

	tr, err := stor.GetTaxRates()
	Expect(err).NotTo(HaveOccurred())
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		GeoRate: georate.New(
			"US",
			"KS",
			"",
			"66212",
			currency.Cents(885),
			currency.Cents(0),
			0,
			currency.Cents(10000),
		),
		TaxShipping: false,
		TaxName:     "TEST TAX",
	})
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		GeoRate: georate.New(
			"US",
			"KS",
			"",
			"",
			currency.Cents(650),
			currency.Cents(0),
			0,
			currency.Cents(10000),
		),
		TaxShipping: false,
		TaxName:     "TEST TAX",
	})
	tr.GeoRates = append(tr.GeoRates, taxrates.GeoRate{
		GeoRate: georate.New(
			"GB",
			"",
			"",
			"",
			currency.Cents(2000),
			currency.Cents(0),
			0,
			currency.Cents(10000),
		),
		TaxShipping: false,
		TaxName:     "VAT",
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
