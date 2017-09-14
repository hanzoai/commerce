package test

import (
	"net/http"
	"testing"
	"time"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/country"
	"hanzo.io/models/types/georate"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	. "hanzo.io/util/test/ginkgo"

	libraryApi "hanzo.io/api/library"
)

func Test(t *testing.T) {
	Setup("api/account", t)
}

var (
	ctx         ae.Context
	cl          *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
)

// Setup appengine context
var _ = BeforeSuite(func() {
	publishedRequired := middleware.TokenRequired(permission.Published)

	// Create a new app engine context
	ctx = ae.NewContext()

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	org = fixtures.Organization(c).(*organization.Organization)

	// Setup client and add routes for account API tests.
	cl = ginclient.New(ctx)
	libraryApi.Route(cl.Router, publishedRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Published)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Add some basic rate date
	stor, err := org.GetDefaultStore()
	Expect(err).NotTo(HaveOccurred())

	sr, err := stor.GetShippingRates()
	Expect(err).NotTo(HaveOccurred())
	sr.GeoRates = append(sr.GeoRates, shippingrates.GeoRate{
		georate.New(
			"",
			"",
			"",
			"",
			0,
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
			0,
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
			0,
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
			0,
		),
		false,
		"VAT",
	})
	tr.MustUpdate()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("library", func() {
	Context("shopjs", func() {
		It("Should get everything", func() {
			req := libraryApi.LoadShopJSReq{}
			res := libraryApi.LoadShopJSRes{}

			log.Debug("Response %s", cl.Post("/library/shopjs", req, &res))
			Expect(len(res.Countries)).To(Equal(len(country.Countries)))
			Expect(res.ShippingRates.StoreId).To(Equal(org.DefaultStore))
			Expect(len(res.ShippingRates.GeoRates)).To(Equal(1))
			Expect(res.TaxRates.StoreId).To(Equal(org.DefaultStore))
			Expect(len(res.TaxRates.GeoRates)).To(Equal(3))
		})

		It("Should get nothing", func() {
			req := libraryApi.LoadShopJSReq{
				HasCountries:     true,
				HasTaxRates:      true,
				HasShippingRates: true,
				LastChecked:      time.Now().Add(2 * time.Hour),
			}
			res := libraryApi.LoadShopJSRes{}

			log.Debug("Response %s", cl.Post("/library/shopjs", req, &res))
			Expect(res.Countries).To(BeNil())
			Expect(res.ShippingRates).To(BeNil())
			Expect(res.TaxRates).To(BeNil())
		})

		It("Should get out of date", func() {
			req := libraryApi.LoadShopJSReq{
				HasCountries:     true,
				HasTaxRates:      true,
				HasShippingRates: true,
				LastChecked:      time.Now().Add(-2 * time.Hour),
			}
			res := libraryApi.LoadShopJSRes{}

			log.Debug("Response %s", cl.Post("/library/shopjs", req, &res))
			Expect(len(res.Countries)).To(Equal(len(country.Countries)))
			Expect(res.ShippingRates.StoreId).To(Equal(org.DefaultStore))
			Expect(len(res.ShippingRates.GeoRates)).To(Equal(1))
			Expect(res.TaxRates.StoreId).To(Equal(org.DefaultStore))
			Expect(len(res.TaxRates.GeoRates)).To(Equal(3))
		})

		It("Should fail for missing store", func() {
			req := libraryApi.LoadShopJSReq{
				StoreId: "123",
			}

			cl.Post("/library/shopjs", req, nil, 404)
		})
	})
})
