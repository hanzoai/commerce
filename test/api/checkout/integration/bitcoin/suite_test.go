package test

import (
	"net/http"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/store"
	"hanzo.io/models/tokensale"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	checkoutApi "hanzo.io/api/checkout"
	storeApi "hanzo.io/api/store"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout/integration/bitcoin", t)
}

var (
	accessToken string
	cl          *ginclient.Client
	ctx         ae.Context
	db          *datastore.Datastore
	org         *organization.Organization
	stor        *store.Store
	u           *user.User
	ts          *tokensale.TokenSale
)

// Setup appengine context
var _ = BeforeSuite(func() {
	// Set BitcoinClient to Test Mode
	bitcoin.Test(true)

	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	fixtures.PlatformWallet(c)

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	stor = fixtures.Store(c).(*store.Store)
	ts = tokensale.Fake(db)
	ts.MustCreate()

	// Setup client and add routes for payment API tests.
	cl = ginclient.New(ctx)
	cl.IgnoreErrors(true)
	checkoutApi.Route(cl.Router, adminRequired)
	storeApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})
