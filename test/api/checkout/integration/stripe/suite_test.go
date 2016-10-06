package test

import (
	"net/http"
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/store"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	checkoutApi "crowdstart.com/api/checkout"
	couponApi "crowdstart.com/api/coupon"
	orderApi "crowdstart.com/api/order"
	storeApi "crowdstart.com/api/store"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe/checkout", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	prod        *product.Product
	stor        *store.Store
	sc          *stripe.Client
	u           *user.User
	refIn       *referrer.Referrer
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run default fixtures to setup organization and store, etc
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	stor = fixtures.Store(c).(*store.Store)
	prod = fixtures.Product(c).(*product.Product)
	fixtures.Variant(c)
	fixtures.Coupon(c)
	fixtures.Discount(c)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	checkoutApi.Route(client.Router, adminRequired)
	orderApi.Route(client.Router, adminRequired)
	storeApi.Route(client.Router, adminRequired)
	couponApi.Route(client.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	org.MustPut()

	// Set authorization header for subsequent requests
	client.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	// Stripe client
	sc = stripe.New(ctx, org.Stripe.Test.AccessToken)

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
