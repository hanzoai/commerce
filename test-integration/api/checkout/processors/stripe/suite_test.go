package integration

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/ginclient"

	checkoutApi "github.com/hanzoai/commerce/api/checkout"
	couponApi "github.com/hanzoai/commerce/api/coupon"
	orderApi "github.com/hanzoai/commerce/api/order"
	storeApi "github.com/hanzoai/commerce/api/store"
	stripeApi "github.com/hanzoai/commerce/thirdparty/stripe/api"

	_ "github.com/hanzoai/commerce/thirdparty/stripe/tasks"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout/integration/stripe", t)
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

	ctx = ae.NewContext(ae.Options{
		Modules: []string{"default"},
		Debug:   true,
	})

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
	cl = ginclient.New(ctx)
	checkoutApi.Route(cl.Router, adminRequired)
	orderApi.Route(cl.Router, adminRequired)
	storeApi.Route(cl.Router, adminRequired)
	couponApi.Route(cl.Router, adminRequired)
	stripeApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken, _ := org.GetTokenByName("test-secret-key")
	org.MustPut()

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
