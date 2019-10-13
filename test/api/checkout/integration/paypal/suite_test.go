package paypal_test

import (
	"net/http"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/user"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	checkoutApi "hanzo.io/api/checkout"
	orderApi "hanzo.io/api/order"
	storeApi "hanzo.io/api/store"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout/integration/paypal", t)
}

var (
	accessToken string
	cl          *ginclient.Client
	ctx         ae.Context
	db          *datastore.Datastore
	org         *organization.Organization
	prod        *product.Product
	refIn       *referrer.Referrer
	stor        *store.Store
	u           *user.User
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)
	prod = fixtures.Product(c).(*product.Product)
	fixtures.Coupon(c)
	fixtures.Variant(c)
	stor = fixtures.Store(c).(*store.Store)

	// Setup client and add routes for payment API tests.
	cl = ginclient.New(ctx)
	checkoutApi.Route(cl.Router, adminRequired)
	orderApi.Route(cl.Router, adminRequired)
	storeApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken, _ := org.GetTokenByName("test-secret-key")
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken.String)
	})

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
