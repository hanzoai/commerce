package test

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/plan"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	subscriptionApi "hanzo.io/api/subscription"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/subscribe", t)
}

var (
	ctx         context.Context
	inst        ae.Instance
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	pln         *plan.Plan
	stor        *store.Store
	sc          *stripe.Client
	u           *user.User
	refIn       *referrer.Referrer
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx, inst, _ = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)
	pln = fixtures.Plan(c).(*plan.Plan)
	fixtures.Coupon(c)
	fixtures.Variant(c)
	stor = fixtures.Store(c).(*store.Store)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	subscriptionApi.Route(client.Router, adminRequired)

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Get apiKey for use with tests
	ap := app.New(db)
	ap.GetById(organization.DefaultAppName)
	apiKey, _, err := ap.GetApiKeyByName(app.TestPublishedKey)
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", apiKey.String)
	})

	sc = stripe.New(ctx, org.Stripe.Test.AccessToken)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})
