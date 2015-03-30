package test

import (
	"net/http"
	"testing"

	"crowdstart.io/api/payment"
	"crowdstart.io/models2/fixtures"
	"crowdstart.io/models2/order"
	"crowdstart.io/test/api/payment/requests"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginclient"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/payment", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
)

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	fixtures.User(c)
	org := fixtures.Organization(c)
	fixtures.Product(c)
	fixtures.Variant(c)

	// Setup client and add routes for payment API
	client = ginclient.New(ctx)
	payment.Route(client.Router)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Authorize", func() {
	It("Should save new order successfully", func() {
		w := client.PostRawJSON("/authorize", requests.ValidOrder)
		ord := order.Order{}

		Expect(w.Code).To(Equal(200))
	})
})
